"""Small embedding clients for Gemini and a clearly isolated offline fallback."""

from __future__ import annotations

import base64
import binascii
import hashlib
import json
import re
import urllib.error
import urllib.parse
import urllib.request
from typing import Any, Protocol


MAX_IMAGE_BYTES = 5 << 20
_IMAGE_HOSTS = frozenset({"cdn.dummyjson.com", "images.unsplash.com"})
_IMAGE_SIGNATURES = {
    "image/jpeg": (b"\xff\xd8\xff",),
    "image/png": (b"\x89PNG\r\n\x1a\n",),
    "image/webp": (b"RIFF",),
}


class ImageDescriber(Protocol):
    def describe_image(self, image_url: str) -> str: ...


class GeminiEmbedding:
    """Gemini multimodal embeddings in one shared text/image vector space."""

    def __init__(
        self,
        api_key: str,
        *,
        model: str = "gemini-embedding-2",
        dimension: int = 768,
        timeout_seconds: float = 15,
    ):
        if not api_key:
            raise ValueError("Gemini API key is required")
        if not 64 <= dimension <= 3072:
            raise ValueError("embedding dimension must be between 64 and 3072")
        self._api_key = api_key
        self.model = model
        self.dimension = dimension
        self.timeout_seconds = timeout_seconds
        self._url = (
            "https://generativelanguage.googleapis.com/v1beta/models/"
            f"{urllib.parse.quote(model, safe='')}"
        )

    def embed_documents(self, texts: list[str]) -> list[list[float]]:
        results: list[list[float]] = []
        for offset in range(0, len(texts), 100):
            batch = texts[offset:offset + 100]
            payload = {
                "requests": [
                    {
                        "model": f"models/{self.model}",
                        "content": {"parts": [{"text": value[:8000]}]},
                        "taskType": "RETRIEVAL_DOCUMENT",
                        "outputDimensionality": self.dimension,
                    }
                    for value in batch
                ]
            }
            response = self._post("batchEmbedContents", payload)
            embeddings = response.get("embeddings", [])
            if len(embeddings) != len(batch):
                raise ValueError("Gemini returned an unexpected embedding count")
            results.extend(self._vector(item) for item in embeddings)
        return results

    def embed_query(self, text: str) -> list[float]:
        response = self._post(
            "embedContent",
            {
                "model": f"models/{self.model}",
                "content": {"parts": [{"text": text[:8000]}]},
                "taskType": "RETRIEVAL_QUERY",
                "outputDimensionality": self.dimension,
            },
        )
        return self._vector(response.get("embedding", {}))

    def embed_image_url(self, image_url: str) -> list[float]:
        mime_type, data = _read_catalog_image(image_url, self.timeout_seconds)
        return self.embed_image(mime_type, data)

    def embed_image_data_url(self, image_url: str) -> list[float]:
        mime_type, data = _decode_image_data_url(image_url)
        return self.embed_image(mime_type, data)

    def embed_image(self, mime_type: str, data: bytes) -> list[float]:
        if len(data) > MAX_IMAGE_BYTES:
            raise ValueError("image exceeds the 5 MiB embedding limit")
        _validate_image(mime_type, data)
        response = self._post(
            "embedContent",
            {
                "model": f"models/{self.model}",
                "content": {
                    "parts": [
                        {
                            "inlineData": {
                                "mimeType": mime_type,
                                "data": base64.b64encode(data).decode("ascii"),
                            }
                        }
                    ]
                },
                "taskType": "RETRIEVAL_QUERY",
                "outputDimensionality": self.dimension,
            },
        )
        return self._vector(response.get("embedding", {}))

    def _post(self, method: str, payload: dict[str, Any]) -> dict[str, Any]:
        request = urllib.request.Request(
            f"{self._url}:{method}",
            data=json.dumps(payload, separators=(",", ":")).encode(),
            headers={
                "Content-Type": "application/json",
                "x-goog-api-key": self._api_key,
                "User-Agent": "GoshopX-Recommender/2.0",
            },
            method="POST",
        )
        try:
            with urllib.request.urlopen(request, timeout=self.timeout_seconds) as response:
                result = json.loads(response.read())
        except urllib.error.HTTPError as exc:
            raise RuntimeError(f"Gemini embedding returned HTTP {exc.code}") from None
        except (urllib.error.URLError, TimeoutError, json.JSONDecodeError) as exc:
            raise RuntimeError(f"Gemini embedding failed: {type(exc).__name__}") from None
        if not isinstance(result, dict):
            raise ValueError("Gemini embedding response must be an object")
        return result

    def _vector(self, embedding: Any) -> list[float]:
        if not isinstance(embedding, dict) or not isinstance(embedding.get("values"), list):
            raise ValueError("Gemini response has no embedding vector")
        values = [float(value) for value in embedding["values"]]
        if len(values) != self.dimension or any(not value == value for value in values):
            raise ValueError("Gemini returned an invalid embedding dimension")
        return values


class GeminiCatalogImageEmbedding:
    """LangChain adapter that stores catalog image vectors, not image URL text."""

    def __init__(self, embeddings: GeminiEmbedding):
        self._embeddings = embeddings
        self.catalog_image_urls = True

    def embed_documents(self, image_urls: list[str]) -> list[list[float]]:
        return [self._embeddings.embed_image_url(value.removeprefix("image:")) for value in image_urls]

    def embed_query(self, text: str) -> list[float]:
        return self._embeddings.embed_query(text)


class HashEmbedding:
    """Offline-only embedding used by isolated tests without provider credentials."""

    def __init__(self, dimension: int = 512):
        if dimension < 64:
            raise ValueError("embedding dimension must be at least 64")
        self.dimension = dimension

    def _embed(self, value: str) -> list[float]:
        vector = [0.0] * self.dimension
        normalized = re.sub(r"\s+", " ", value.casefold().strip())
        tokens = re.findall(r"[\wÀ-ỹ]+", normalized)
        features = tokens + [normalized[index:index + 3] for index in range(max(0, len(normalized) - 2))]
        if not features:
            return vector
        for feature in features:
            digest = hashlib.blake2b(feature.encode("utf-8"), digest_size=8).digest()
            index = int.from_bytes(digest[:4], "big") % self.dimension
            vector[index] += 1.0 if digest[4] & 1 else -1.0
        norm = sum(item * item for item in vector) ** 0.5
        return [item / norm for item in vector] if norm else vector

    def embed_documents(self, texts: list[str]) -> list[list[float]]:
        return [self._embed(text) for text in texts]

    def embed_query(self, text: str) -> list[float]:
        return self._embed(text)


def _decode_image_data_url(value: str) -> tuple[str, bytes]:
    match = re.fullmatch(
        r"data:(image/(?:jpeg|png|webp));base64,([A-Za-z0-9+/]*={0,2})",
        str(value or ""),
    )
    if not match:
        raise ValueError("image must be a base64 JPEG, PNG, or WebP data URL")
    try:
        data = base64.b64decode(match.group(2), validate=True)
    except (binascii.Error, ValueError) as exc:
        raise ValueError("image data URL is invalid") from exc
    _validate_image(match.group(1), data)
    return match.group(1), data


def _read_catalog_image(image_url: str, timeout: float) -> tuple[str, bytes]:
    parsed = urllib.parse.urlparse(image_url)
    if parsed.scheme != "https" or parsed.hostname not in _IMAGE_HOSTS or parsed.username or parsed.password:
        raise ValueError("catalog image host is not allowlisted")
    request = urllib.request.Request(image_url, headers={"User-Agent": "GoshopX/2.0"})
    class _NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, headers, newurl):
            return None
    try:
        with urllib.request.build_opener(_NoRedirect).open(request, timeout=timeout) as response:
            data = response.read(MAX_IMAGE_BYTES + 1)
            mime_type = response.headers.get_content_type()
    except urllib.error.URLError as exc:
        raise RuntimeError(f"catalog image fetch failed: {type(exc).__name__}") from None
    if len(data) > MAX_IMAGE_BYTES:
        raise ValueError("catalog image exceeds the 5 MiB limit")
    _validate_image(mime_type, data)
    return mime_type, data


def _validate_image(mime_type: str, data: bytes) -> None:
    signatures = _IMAGE_SIGNATURES.get(mime_type)
    valid = bool(data) and (
        any(data.startswith(signature) for signature in signatures)
        if mime_type != "image/webp"
        else len(data) >= 12 and data.startswith(b"RIFF") and data[8:12] == b"WEBP"
    )
    if not valid:
        raise ValueError("image bytes do not match a supported JPEG, PNG, or WebP image")


def describe_product_image(image_url: str, llm: ImageDescriber) -> str:
    """Ask the configured vision API for visible catalog attributes only."""
    description = llm.describe_image(image_url).strip()
    return description[:2000]

from __future__ import annotations

import csv
import io
import json
import re
import zipfile
from xml.etree import ElementTree

MAX_DOCUMENT_BYTES = 20 * 1024 * 1024
MAX_PDF_PAGES = 200
CHUNK_SIZE = 800
CHUNK_OVERLAP = 120
_CONTENT_TYPES = {
    ".pdf": {"application/pdf"},
    ".docx": {"application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
    ".txt": {"text/plain"},
    ".md": {"text/markdown", "text/plain"},
    ".json": {"application/json", "text/json"},
    ".csv": {"text/csv", "application/csv", "text/plain"},
}


class DocumentParseError(ValueError):
    pass


def parse_document(name: str, content_type: str, data: bytes) -> list[tuple[str, dict]]:
    """Extract bounded, provenance-tagged chunks from supported admin documents."""
    if not data or len(data) > MAX_DOCUMENT_BYTES:
        raise DocumentParseError("document_size_invalid")
    extension = name.lower().rsplit(".", 1)[-1] if "." in name else ""
    extension = f".{extension}"
    if extension not in _CONTENT_TYPES or content_type.lower().split(";", 1)[0].strip() not in _CONTENT_TYPES[extension]:
        raise DocumentParseError("document_type_mismatch")

    if extension in {".txt", ".md"}:
        text = _decode_text(data)
        sections = [(text, {"format": extension[1:]})]
    elif extension == ".csv":
        try:
            rows = list(csv.DictReader(io.StringIO(_decode_text(data))))
            if len(rows) > 10_000:
                raise DocumentParseError("csv_row_limit")
        except (csv.Error, UnicodeError):
            raise DocumentParseError("csv_invalid") from None
        if not rows:
            raise DocumentParseError("document_has_no_text")
        sections = [(json.dumps(row, ensure_ascii=False, sort_keys=True), {"format": "csv", "row": index + 1})
                    for index, row in enumerate(rows)]
    elif extension == ".json":
        try:
            value = json.loads(_decode_text(data))
        except (json.JSONDecodeError, UnicodeError):
            raise DocumentParseError("json_invalid") from None
        sections = [(json.dumps(value, ensure_ascii=False, sort_keys=True), {"format": "json"})]
    elif extension == ".docx":
        sections = _parse_docx(data)
    else:
        sections = _parse_pdf(data)

    chunks: list[tuple[str, dict]] = []
    for section, metadata in sections:
        normalized = re.sub(r"\s+", " ", section).strip()
        if not normalized:
            continue
        start = 0
        while start < len(normalized):
            end = min(start + CHUNK_SIZE, len(normalized))
            chunk = normalized[start:end].strip()
            if chunk:
                chunks.append((chunk, {**metadata, "start": start, "end": end}))
            if end == len(normalized):
                break
            start = end - CHUNK_OVERLAP
    if not chunks:
        raise DocumentParseError("document_has_no_text")
    return chunks


def _decode_text(data: bytes) -> str:
    try:
        return data.decode("utf-8-sig")
    except UnicodeDecodeError:
        raise DocumentParseError("text_encoding_unsupported") from None


def _parse_docx(data: bytes) -> list[tuple[str, dict]]:
    if b"<!DOCTYPE" in data.upper() or b"<!ENTITY" in data.upper():
        raise DocumentParseError("docx_xml_entities_rejected")
    try:
        with zipfile.ZipFile(io.BytesIO(data)) as archive:
            info = archive.getinfo("word/document.xml")
            if info.file_size > MAX_DOCUMENT_BYTES or (info.compress_size and info.file_size / info.compress_size > 100):
                raise DocumentParseError("docx_expansion_limit")
            root = ElementTree.fromstring(archive.read(info))
    except DocumentParseError:
        raise
    except (KeyError, zipfile.BadZipFile, ElementTree.ParseError, OSError):
        raise DocumentParseError("docx_invalid") from None
    namespace = {"w": "http://schemas.openxmlformats.org/wordprocessingml/2006/main"}
    paragraphs = ["".join(node.itertext()) for node in root.findall(".//w:p", namespace)]
    text = "\n".join(paragraph for paragraph in paragraphs if paragraph.strip())
    return [(text, {"format": "docx", "section": "document"})]


def _parse_pdf(data: bytes) -> list[tuple[str, dict]]:
    try:
        from pypdf import PdfReader
    except ImportError:
        raise DocumentParseError("pdf_parser_unavailable") from None
    try:
        reader = PdfReader(io.BytesIO(data), strict=True)
        if reader.is_encrypted:
            raise DocumentParseError("pdf_encrypted")
        if len(reader.pages) > MAX_PDF_PAGES:
            raise DocumentParseError("pdf_page_limit")
        sections = [(page.extract_text() or "", {"format": "pdf", "page": index + 1})
                    for index, page in enumerate(reader.pages)]
    except DocumentParseError:
        raise
    except Exception:
        raise DocumentParseError("pdf_invalid_or_unsupported") from None
    if not any(text.strip() for text, _ in sections):
        raise DocumentParseError("pdf_requires_ocr")
    return sections

from __future__ import annotations

from collections import Counter
from dataclasses import dataclass
import math
import re


def tokenize(value: str) -> list[str]:
    return [token.casefold() for token in re.findall(r"[\w]+", value, flags=re.UNICODE) if len(token) > 1]


@dataclass(frozen=True)
class BM25Index:
    documents: dict[str, list[str]]
    document_frequency: Counter[str]
    average_length: float

    @classmethod
    def build(cls, documents: dict[str, list[str]]) -> "BM25Index":
        frequency = Counter(token for document in documents.values() for token in set(document))
        average = sum(map(len, documents.values())) / max(1, len(documents))
        return cls(documents, frequency, average)

    def score(self, query_tokens: set[str], product_id: str) -> float:
        return score(query_tokens, self.documents, self.document_frequency, product_id, self.average_length)


def score(query_tokens: set[str], documents: dict[str, list[str]], document_frequency: Counter[str], product_id: str, average_length: float | None = None) -> float:
    doc = documents[product_id]
    frequencies = Counter(doc)
    average_length = average_length if average_length is not None else sum(map(len, documents.values())) / max(1, len(documents))
    result = 0.0
    for token in query_tokens:
        frequency = frequencies[token]
        if frequency:
            doc_frequency = document_frequency[token]
            idf = math.log(1 + (len(documents) - doc_frequency + 0.5) / (doc_frequency + 0.5))
            result += idf * frequency * 2.5 / (frequency + 1.5 * (0.25 + 0.75 * len(doc) / max(average_length, 1)))
    return result

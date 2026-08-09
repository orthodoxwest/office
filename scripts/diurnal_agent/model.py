"""Schemas and validation for advisory diurnal review results."""

from __future__ import annotations

import hashlib
import json
import re
from typing import Any


SCHEMA_VERSION = 1
SAFE_VERDICTS = {
    "match",
    "replace-existing",
    "missing-override",
    "fallback-equal",
    "needs-modeling",
    "needs-ruling",
    "ambiguous",
}

RESULT_SCHEMA = {
    "type": "object",
    "additionalProperties": False,
    "required": [
        "verdict",
        "confidence",
        "rationale",
        "target_key",
        "evidence",
        "review_questions",
    ],
    "properties": {
        "verdict": {"type": "string", "enum": sorted(SAFE_VERDICTS)},
        "confidence": {"type": "number", "minimum": 0, "maximum": 1},
        "rationale": {"type": "string", "maxLength": 4000},
        "target_key": {"type": ["string", "null"], "maxLength": 300},
        "evidence": {
            "type": "array",
            "maxItems": 20,
            "items": {
                "type": "object",
                "additionalProperties": False,
                "required": ["claim", "location"],
                "properties": {
                    "claim": {"type": "string", "maxLength": 1000},
                    "location": {"type": "string", "maxLength": 500},
                },
            },
        },
        "review_questions": {
            "type": "array",
            "maxItems": 10,
            "items": {"type": "string", "maxLength": 1000},
        },
    },
}


def canonical_json(value: Any) -> str:
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False)


def digest(value: Any) -> str:
    if isinstance(value, bytes):
        payload = value
    elif isinstance(value, str):
        payload = value.encode()
    else:
        payload = canonical_json(value).encode()
    return hashlib.sha256(payload).hexdigest()


def safe_job_id(candidate_id: str, candidate: dict[str, Any]) -> str:
    slug = re.sub(r"[^A-Za-z0-9._-]+", "-", candidate_id).strip("-.")[:80]
    return f"{slug or 'candidate'}-{digest(candidate)[:12]}"


def validate_result(value: Any) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise ValueError("proposal must be a JSON object")
    allowed = set(RESULT_SCHEMA["properties"])
    required = set(RESULT_SCHEMA["required"])
    if set(value) - allowed or not required <= set(value):
        raise ValueError("proposal does not match the advisory schema")
    if value["verdict"] not in SAFE_VERDICTS:
        raise ValueError("invalid proposal verdict")
    confidence = value["confidence"]
    if isinstance(confidence, bool) or not isinstance(confidence, (int, float)) or not 0 <= confidence <= 1:
        raise ValueError("confidence must be a number from zero to one")
    if not isinstance(value["rationale"], str) or len(value["rationale"]) > 4000:
        raise ValueError("invalid rationale")
    target = value.get("target_key")
    if target is not None and (not isinstance(target, str) or len(target) > 300):
        raise ValueError("invalid target key")
    evidence = value["evidence"]
    if not isinstance(evidence, list) or len(evidence) > 20:
        raise ValueError("invalid evidence list")
    for item in evidence:
        if not isinstance(item, dict) or set(item) != {"claim", "location"}:
            raise ValueError("evidence must contain claim and location")
        if not isinstance(item["claim"], str) or len(item["claim"]) > 1000:
            raise ValueError("invalid evidence claim")
        if not isinstance(item["location"], str) or len(item["location"]) > 500:
            raise ValueError("invalid evidence location")
    questions = value.get("review_questions", [])
    if not isinstance(questions, list) or len(questions) > 10 or any(
        not isinstance(item, str) or len(item) > 1000 for item in questions
    ):
        raise ValueError("invalid review questions")
    lowered = canonical_json(value).lower()
    forbidden = ("\x60\x60\x60patch", "git apply", "rm -rf", "decisions.csv", "provenance.csv")
    if any(token in lowered for token in forbidden):
        raise ValueError("proposal contains mutation instructions")
    return value

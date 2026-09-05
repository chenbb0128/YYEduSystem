#!/usr/bin/env python3
"""Small local RapidOCR HTTP service for 豆芽成长助手.

Install:
    python -m pip install rapidocr onnxruntime

Run:
    python scripts/rapidocr_service.py --host 127.0.0.1 --port 9009

Request:
    POST /ocr
    {"image_base64":"...","content_type":"image/jpeg"}

Response:
    {"code":0,"provider":"rapidocr","text":"...","lines":[{"text":"...","confidence":0.98}]}
"""

from __future__ import annotations

import argparse
import base64
import json
import os
import tempfile
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any


ENGINE: Any | None = None


def load_engine() -> Any:
    global ENGINE
    if ENGINE is not None:
        return ENGINE
    try:
        from rapidocr import RapidOCR
    except ImportError:
        from rapidocr_onnxruntime import RapidOCR
    ENGINE = RapidOCR()
    return ENGINE


def suffix_for_content_type(content_type: str) -> str:
    content_type = (content_type or "").lower()
    if "png" in content_type:
        return ".png"
    if "webp" in content_type:
        return ".webp"
    return ".jpg"


def extract_text(image_bytes: bytes, content_type: str) -> dict[str, Any]:
    engine = load_engine()
    suffix = suffix_for_content_type(content_type)
    temp_path = ""
    try:
        with tempfile.NamedTemporaryFile(delete=False, suffix=suffix) as image_file:
            image_file.write(image_bytes)
            temp_path = image_file.name
        raw_result = engine(temp_path)
        lines = normalize_result(raw_result)
        text = "\n".join(item["text"] for item in lines if item["text"])
        return {"code": 0, "provider": "rapidocr", "text": text, "lines": lines}
    finally:
        if temp_path:
            try:
                os.remove(temp_path)
            except OSError:
                pass


def normalize_result(raw_result: Any) -> list[dict[str, Any]]:
    if raw_result is None:
        return []
    if isinstance(raw_result, tuple) and raw_result:
        raw_result = raw_result[0]
    attr_lines = lines_from_attrs(raw_result)
    if attr_lines:
        return attr_lines
    if hasattr(raw_result, "to_json"):
        try:
            raw_result = raw_result.to_json()
        except TypeError:
            pass
    if isinstance(raw_result, str):
        try:
            raw_result = json.loads(raw_result)
        except json.JSONDecodeError:
            return [{"text": line.strip(), "confidence": 0} for line in raw_result.splitlines() if line.strip()]
    if isinstance(raw_result, dict):
        lines = lines_from_dict(raw_result)
        if lines:
            return lines
        text = raw_result.get("text") or raw_result.get("result") or ""
        if isinstance(text, str):
            return [{"text": line.strip(), "confidence": 0} for line in text.splitlines() if line.strip()]
        return []
    return lines_from_sequence(raw_result)


def lines_from_attrs(raw: Any) -> list[dict[str, Any]]:
    txts = getattr(raw, "txts", None)
    scores = getattr(raw, "scores", None)
    if not isinstance(txts, (list, tuple)):
        return []
    out = []
    for index, text in enumerate(txts):
        confidence = scores[index] if isinstance(scores, (list, tuple)) and index < len(scores) else 0
        out.append({"text": str(text).strip(), "confidence": safe_score(confidence)})
    return [item for item in out if item["text"]]


def lines_from_dict(raw: dict[str, Any]) -> list[dict[str, Any]]:
    txts = raw.get("txts") or raw.get("texts")
    scores = raw.get("scores") or raw.get("confidences") or []
    if isinstance(txts, list):
        out = []
        for index, text in enumerate(txts):
            confidence = scores[index] if isinstance(scores, list) and index < len(scores) else 0
            out.append({"text": str(text).strip(), "confidence": safe_score(confidence)})
        return [item for item in out if item["text"]]
    for key in ("lines", "items", "results"):
        value = raw.get(key)
        if isinstance(value, list):
            return lines_from_sequence(value)
    return []


def lines_from_sequence(raw: Any) -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []
    if not isinstance(raw, (list, tuple)):
        return out
    for item in raw:
        parsed = parse_line(item)
        if parsed and parsed["text"]:
            out.append(parsed)
    return out


def parse_line(item: Any) -> dict[str, Any] | None:
    if item is None:
        return None
    if isinstance(item, dict):
        text = item.get("text") or item.get("detected_text") or item.get("DetectedText") or ""
        confidence = item.get("confidence") or item.get("score") or item.get("Confidence") or 0
        return {"text": str(text).strip(), "confidence": safe_score(confidence)}
    if isinstance(item, str):
        return {"text": item.strip(), "confidence": 0}
    if isinstance(item, (list, tuple)):
        # rapidocr >= 3 often exposes (text, confidence, box). Older
        # rapidocr_onnxruntime commonly exposes (box, text, confidence).
        if len(item) >= 3:
            if isinstance(item[0], str):
                return {"text": item[0].strip(), "confidence": safe_score(item[1])}
            return {"text": str(item[1]).strip(), "confidence": safe_score(item[2])}
        if len(item) >= 1:
            return {"text": str(item[0]).strip(), "confidence": 0}
    return None


def safe_score(value: Any) -> float:
    try:
        score = float(value)
    except (TypeError, ValueError):
        return 0
    if score > 1:
        score = score / 100
    if score < 0:
        return 0
    if score > 1:
        return 1
    return score


class OCRHandler(BaseHTTPRequestHandler):
    server_version = "XingyaRapidOCR/1.0"

    def do_GET(self) -> None:
        if self.path.rstrip("/") in ("", "/healthz"):
            self.write_json(HTTPStatus.OK, {"code": 0, "message": "ok"})
            return
        self.write_json(HTTPStatus.NOT_FOUND, {"code": 404, "message": "not found"})

    def do_POST(self) -> None:
        if self.path != "/ocr":
            self.write_json(HTTPStatus.NOT_FOUND, {"code": 404, "message": "not found"})
            return
        try:
            content_length = int(self.headers.get("Content-Length", "0"))
            if content_length <= 0:
                raise ValueError("empty request body")
            payload = json.loads(self.rfile.read(content_length).decode("utf-8"))
            image_base64 = payload.get("image_base64") or payload.get("ImageBase64") or ""
            content_type = payload.get("content_type") or payload.get("ContentType") or "image/jpeg"
            if not image_base64:
                raise ValueError("image_base64 is required")
            image_bytes = base64.b64decode(image_base64, validate=True)
            if not image_bytes:
                raise ValueError("image is empty")
            self.write_json(HTTPStatus.OK, extract_text(image_bytes, content_type))
        except Exception as exc:  # noqa: BLE001 - service boundary returns safe JSON
            self.write_json(HTTPStatus.OK, {"code": 1, "message": str(exc), "provider": "rapidocr", "text": "", "lines": []})

    def log_message(self, format: str, *args: Any) -> None:
        return

    def write_json(self, status: HTTPStatus, payload: dict[str, Any]) -> None:
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def main() -> None:
    parser = argparse.ArgumentParser(description="Run local RapidOCR HTTP service.")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", default=9009, type=int)
    args = parser.parse_args()
    load_engine()
    server = ThreadingHTTPServer((args.host, args.port), OCRHandler)
    print(f"RapidOCR service listening on http://{args.host}:{args.port}/ocr")
    server.serve_forever()


if __name__ == "__main__":
    main()

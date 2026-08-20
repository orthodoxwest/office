#!/usr/bin/env python3
"""Resumable, evidence-preserving intake for a printed diurnal PDF."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import shutil
import subprocess
import tempfile
import xml.etree.ElementTree as ET
import math
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_OUTPUT = ROOT / "output" / "diurnal-ingest"
ATTENTION_ROUTES = {"weak-native", "mixed-review", "ocr-unavailable", "ocr-failed"}
SOURCE_KEY_PATTERN = re.compile(r"[a-z0-9][a-z0-9-]*")


def sha(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def now() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def which(command: str) -> str | None:
    return shutil.which(command)


def cmd(argv: list[str], text: bool = False) -> subprocess.CompletedProcess:
    return subprocess.run(argv, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=text, check=False)


def atomic(path: Path, data: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(fd, "wb") as handle:
            handle.write(data)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def dump(path: Path, value: object) -> None:
    atomic(path, (json.dumps(value, indent=2, sort_keys=True) + "\n").encode())


def load(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def key(pdf: Path) -> str:
    return re.sub("[^a-z0-9]+", "-", pdf.stem.lower()).strip("-") or "diurnal"


def pages(pdf: Path) -> int:
    result = cmd(["pdfinfo", str(pdf)], text=True)
    match = re.search(r"^Pages:\s+(\d+)", result.stdout, re.MULTILINE)
    if result.returncode or not match:
        raise RuntimeError("pdfinfo did not report page count")
    return int(match.group(1))


def metrics(text: str) -> dict[str, int | float]:
    visible_size = max(1, sum(not char.isspace() for char in text))
    lines = [line for line in text.splitlines() if line.strip()]
    letters = sum(char.isalpha() for char in text)
    glyph_noise = sum(
        ord(char) == 0xFFFD or 0xE000 <= ord(char) <= 0xF8FF
        for char in text
    )
    return {
        "chars": len(text),
        "visible_chars": visible_size,
        "letters": letters,
        "alpha_ratio": round(letters / visible_size, 4),
        "replacement_or_pua": glyph_noise,
        "glyph_noise_ratio": round(glyph_noise / visible_size, 4),
        "controls": sum(ord(char) < 32 and char not in "\n\r\t\f" for char in text),
        "nonblank_lines": len(lines),
        "longest_line": max((len(line) for line in lines), default=0),
    }


def route(page_metrics: dict[str, int | float]) -> str:
    if (
        page_metrics["letters"] < 80
        or page_metrics["alpha_ratio"] < 0.20
        or page_metrics["glyph_noise_ratio"] > 0.05
        or page_metrics["controls"]
    ):
        return "weak-native"
    return "native"


def manifest_path(run: Path) -> Path:
    return run / "manifest.json"


def require_output_path(path: Path) -> Path:
    """Confine source-derived artifacts to the repository's ignored output."""
    resolved = path.resolve()
    allowed = (ROOT / "output").resolve()
    try:
        resolved.relative_to(allowed)
    except ValueError as exc:
        raise SystemExit(f"diurnal artifacts must stay below {allowed}") from exc
    return resolved


def manifest(run: Path) -> dict:
    path = manifest_path(run)
    if not path.exists():
        raise SystemExit("no manifest; run register first")
    return load(path)


def register(pdf: Path, output: Path, source_key: str | None = None) -> Path:
    if not pdf.is_file():
        raise SystemExit(f"PDF not found: {pdf}")
    if not which("pdfinfo") or not which("pdftotext"):
        raise SystemExit("Poppler pdfinfo and pdftotext are required")
    selected_key = source_key or key(pdf)
    if not SOURCE_KEY_PATTERN.fullmatch(selected_key):
        raise SystemExit("source key must contain only lowercase letters, numbers, and hyphens")
    source_hash = sha(pdf.read_bytes())
    run = output / f"{selected_key}-{source_hash[:12]}"
    run.mkdir(parents=True, exist_ok=True)
    if manifest_path(run).exists():
        return run
    dump(manifest_path(run), {
        "schema": 1, "created_at": now(), "pdf": str(pdf.resolve()),
        "source_key": selected_key, "source_sha256": source_hash,
        "page_count": pages(pdf), "run_id": f"DI-{source_hash[:12]}", "pages": {},
    })
    return run


def validate_source(run_manifest: dict) -> Path:
    pdf = Path(run_manifest["pdf"])
    if not pdf.is_file() or sha(pdf.read_bytes()) != run_manifest.get("source_sha256"):
        raise SystemExit("source PDF no longer matches manifest; register a new run")
    return pdf


def split_whole_document(whole: Path, page_count: int) -> list[str] | None:
    parts = whole.read_text(encoding="utf-8", errors="replace").split("\f")
    if parts and not parts[-1].strip():
        parts.pop()
    return parts if len(parts) == page_count else None


def load_column_spec(path: Path) -> dict:
    """Load an explicit, page-scoped column map; never infer columns."""
    payload = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(payload, dict) or not isinstance(payload.get("pages"), dict):
        raise SystemExit("column spec must be an object with a pages map")
    for page, columns in payload["pages"].items():
        if not str(page).isdigit() or int(page) < 1 or not isinstance(columns, list) or not columns:
            raise SystemExit("column spec pages must map page numbers to non-empty lists")
        page_labels = set()
        boxes = []
        for item in columns:
            label = item.get("column") if isinstance(item, dict) else None
            if not isinstance(label, str) or not re.fullmatch(r"[a-z][a-z0-9_-]*", label) or label in page_labels:
                raise SystemExit("each column must declare a string column name")
            page_labels.add(label)
            bbox = item.get("bbox")
            if (not isinstance(bbox, list) or len(bbox) != 4 or
                any(isinstance(v, bool) or not isinstance(v, (int, float)) or not math.isfinite(v) for v in bbox)):
                raise SystemExit("each column must declare numeric bbox [x0,y0,x1,y1]")
            if bbox[2] <= bbox[0] or bbox[3] <= bbox[1]:
                raise SystemExit("column bbox must have positive width and height")
            boxes.append(bbox)
        for index, left in enumerate(boxes):
            for right in boxes[index + 1:]:
                if min(left[2], right[2]) > max(left[0], right[0]) and min(left[3], right[3]) > max(left[1], right[1]):
                    raise SystemExit("column bboxes may not overlap")
    return payload


def extract_page_columns(pdf: Path, page: int, page_dir: Path, specs: list[dict]) -> list[dict]:
    """Materialize explicitly bounded column witnesses from pdftotext bboxes."""
    result = cmd(["pdftotext", "-bbox-layout", "-f", str(page), "-l", str(page), str(pdf), "-"])
    if result.returncode:
        raise SystemExit(f"pdftotext bbox extraction failed page {page}")
    raw = result.stdout.decode("utf-8", errors="replace") if isinstance(result.stdout, bytes) else result.stdout
    try:
        root = ET.fromstring(raw)
    except ET.ParseError as exc:
        raise SystemExit(f"pdftotext bbox output is invalid on page {page}") from exc
    words = []
    for node in root.iter():
        if node.tag.rsplit("}", 1)[-1] != "word":
            continue
        try:
            x0, y0 = float(node.attrib["xMin"]), float(node.attrib["yMin"])
            x1, y1 = float(node.attrib["xMax"]), float(node.attrib["yMax"])
        except (KeyError, ValueError):
            continue
        words.append((x0, y0, x1, y1, "".join(node.itertext()).strip()))
    artifacts = []
    for spec in specs:
        name = spec["column"]
        x0, y0, x1, y1 = map(float, spec["bbox"])
        selected = [word for word in words if word[4] and word[0] >= x0 and word[2] <= x1 and word[1] >= y0 and word[3] <= y1]
        lines = {}
        for wx0, wy0, wx1, wy1, text in selected:
            key = round(wy0, 1)
            lines.setdefault(key, []).append((wx0, text))
        flattened = "\n".join(" ".join(text for _x, text in sorted(items)) for _y, items in sorted(lines.items()))
        path = page_dir / "columns" / f"{name}.txt"
        atomic(path, flattened.encode())
        artifacts.append({
            "column": name, "bbox": [x0, y0, x1, y1],
            "text_path": str(path.relative_to(page_dir.parent.parent)),
            "raw_text_sha256": sha(flattened.encode()),
            "extractor": "pdftotext-bbox-layout-column",
            "canonical_text": flattened,
        })
    return artifacts


def extract(run: Path, resume: bool = False, column_spec: Path | None = None) -> None:
    run_manifest = manifest(run)
    pdf = validate_source(run_manifest)
    if column_spec is not None:
        spec_bytes = column_spec.read_bytes()
        spec_hash = sha(spec_bytes)
        if resume and run_manifest.get("column_spec_sha256") and run_manifest["column_spec_sha256"] != spec_hash:
            raise SystemExit("column spec changed; register a new run or remove the incomplete run")
        run_manifest["columns"] = load_column_spec(column_spec)
        run_manifest["column_spec_sha256"] = spec_hash
    for page_key in run_manifest.get("columns", {}).get("pages", {}):
        if int(page_key) > int(run_manifest["page_count"]):
            raise SystemExit("column spec references a page outside the PDF")
    whole = run / "native-all.txt"
    if not (resume and whole.exists()):
        result = cmd(["pdftotext", "-layout", str(pdf), str(whole)])
        if result.returncode:
            raise SystemExit("pdftotext failed: " + result.stderr.decode(errors="replace"))
    parts = split_whole_document(whole, run_manifest["page_count"])
    for page_number in range(1, run_manifest["page_count"] + 1):
        page_dir = run / "pages" / f"{page_number:04d}"
        text_path = page_dir / "native.layout.txt"
        page_dir.mkdir(parents=True, exist_ok=True)
        if not (resume and text_path.exists()):
            if parts is not None:
                atomic(text_path, parts[page_number - 1].encode())
            else:
                result = cmd(["pdftotext", "-layout", "-f", str(page_number), "-l", str(page_number), str(pdf), str(text_path)])
                if result.returncode:
                    raise SystemExit(f"pdftotext failed page {page_number}")
        text = text_path.read_text(encoding="utf-8", errors="replace")
        page_metrics = metrics(text)
        record = {
            "page": page_number,
            "id": f"{run_manifest['run_id']}-P{page_number:04d}-{sha(text.encode())[:12]}",
            "source_key": run_manifest["source_key"], "source_sha256": run_manifest["source_sha256"],
            "native_path": str(text_path.relative_to(run)), "text_path": str(text_path.relative_to(run)),
            "native_text_sha256": sha(text.encode()), "raw_text_sha256": sha(text.encode()),
            "metrics": page_metrics, "route": route(page_metrics),
            "extractor": "pdftotext-layout", "whole_document": parts is not None,
        }
        page_specs = run_manifest.get("columns", {}).get("pages", {}).get(str(page_number), [])
        if page_specs:
            if record["route"] != "native":
                raise SystemExit(f"explicit columns require a native page witness (page {page_number} is {record['route']})")
            record["columns"] = extract_page_columns(pdf, page_number, page_dir, page_specs)
        dump(page_dir / "page.json", record)
        run_manifest["pages"][str(page_number)] = record
    run_manifest["extracted_at"] = now()
    run_manifest["whole_document_page_count_matched"] = parts is not None
    dump(manifest_path(run), run_manifest)


def choose(run_manifest: dict, selection: str) -> list[int]:
    all_pages = range(1, run_manifest["page_count"] + 1)
    if selection == "all":
        return list(all_pages)
    if selection in {"weak", "mixed"}:
        return [page for page in all_pages if run_manifest["pages"].get(str(page), {}).get("route") in {"weak-native", "mixed-review"}]
    if selection == "ocr":
        retryable = {"weak-native", "mixed-review", "ocr-unavailable", "ocr-failed"}
        return [
            page
            for page in all_pages
            if run_manifest["pages"].get(str(page), {}).get("route") in retryable
        ]
    selected: set[int] = set()
    for token in selection.split(","):
        bounds = token.split("-", 1)
        start = int(bounds[0])
        end = int(bounds[-1])
        selected.update(range(start, end + 1))
    return sorted(selected)


def render(run: Path, selection: str = "weak", dpi: int = 300) -> None:
    if not which("pdftoppm"):
        raise SystemExit("pdftoppm is required to render pages")
    run_manifest = manifest(run)
    pdf = validate_source(run_manifest)
    for page_number in choose(run_manifest, selection):
        page_dir = run / "pages" / f"{page_number:04d}"
        image = page_dir / "render.png"
        result = cmd(["pdftoppm", "-png", "-r", str(dpi), "-f", str(page_number), "-l", str(page_number), "-singlefile", str(pdf), str(image.with_suffix(""))])
        if result.returncode:
            raise SystemExit(f"pdftoppm failed page {page_number}")
        record = run_manifest["pages"][str(page_number)]
        record.update({"render_path": str(image.relative_to(run)), "render_dpi": dpi})
        if record["route"] == "weak-native":
            record["route"] = "mixed-review"
        dump(page_dir / "page.json", record)
    dump(manifest_path(run), run_manifest)


def ocr(run: Path, lang: str = "eng", psm: int = 6) -> int:
    run_manifest = manifest(run)
    selected = choose(run_manifest, "ocr")
    if not which("tesseract"):
        for page_number in selected:
            record = run_manifest["pages"][str(page_number)]
            record["route"] = "ocr-unavailable"
            dump(run / "pages" / f"{page_number:04d}" / "page.json", record)
        dump(manifest_path(run), run_manifest)
        return 0
    missing_images = [page for page in selected if not (run / "pages" / f"{page:04d}" / "render.png").exists()]
    if missing_images:
        raise SystemExit("OCR needs render.png; run render first (missing pages: " + ", ".join(map(str, missing_images)) + ")")
    for page_number in selected:
        page_dir = run / "pages" / f"{page_number:04d}"
        image, stem = page_dir / "render.png", page_dir / "ocr"
        record = run_manifest["pages"][str(page_number)]
        if record.get("ocr_error"):
            record.setdefault("ocr_attempts", []).append({
                "route": record.get("route"),
                "error": record["ocr_error"],
                "retried_at": now(),
            })
        result = cmd(["tesseract", str(image), str(stem), "-l", lang, "--psm", str(psm)])
        if result.returncode:
            record.update({"route": "ocr-failed", "ocr_error": result.stderr.decode(errors="replace")})
        else:
            tsv_result = cmd(["tesseract", str(image), str(stem), "-l", lang, "--psm", str(psm), "tsv"])
            text_path = stem.with_suffix(".txt")
            if tsv_result.returncode or not text_path.exists():
                error = tsv_result.stderr if tsv_result.returncode else b"missing OCR text output"
                record.update({"route": "ocr-failed", "ocr_error": error.decode(errors="replace")})
            else:
                ocr_text = text_path.read_text(encoding="utf-8", errors="replace")
                record.update({
                    "route": "ocr",
                    "extractor": "tesseract",
                    "ocr_path": str(text_path.relative_to(run)),
                    "text_path": str(text_path.relative_to(run)),
                    "raw_text_sha256": sha(ocr_text.encode()),
                    "ocr_metrics": metrics(ocr_text),
                })
                record.pop("ocr_error", None)
                record["id"] = (
                    f"{run_manifest['run_id']}-P{page_number:04d}-"
                    f"{record['raw_text_sha256'][:12]}"
                )
        dump(page_dir / "page.json", record)
    dump(manifest_path(run), run_manifest)
    return 0


def mark_ocr_unavailable(run: Path, reason: str) -> None:
    run_manifest = manifest(run)
    for page_number in choose(run_manifest, "mixed"):
        record = run_manifest["pages"][str(page_number)]
        record.update({"route": "ocr-unavailable", "ocr_error": reason})
        dump(run / "pages" / f"{page_number:04d}" / "page.json", record)
    dump(manifest_path(run), run_manifest)


def qa(run: Path) -> int:
    run_manifest = manifest(run)
    counts: dict[str, int] = {}
    for record in run_manifest["pages"].values():
        counts[record["route"]] = counts.get(record["route"], 0) + 1
    enumerated = len(run_manifest["pages"]) == run_manifest["page_count"]
    needs_attention = sum(value for name, value in counts.items() if name in ATTENTION_ROUTES)
    report = {"run_id": run_manifest["run_id"], "page_count": run_manifest["page_count"], "extracted": len(run_manifest["pages"]), "routes": counts, "needs_attention": needs_attention, "complete": enumerated and not needs_attention, "partial": not enumerated or bool(needs_attention)}
    dump(run / "qa.json", report)
    print(json.dumps(report, indent=2))
    return 0 if report["complete"] else 2


def doctor(json_output: bool = False, ocr_required: bool = False) -> int:
    availability = {name: bool(which(name)) for name in ("pdfinfo", "pdftotext", "pdftoppm", "tesseract")}
    ocr_ready = availability["pdftoppm"] and availability["tesseract"]
    availability["ocr_ready"] = ocr_ready
    availability["ready"] = (
        availability["pdfinfo"]
        and availability["pdftotext"]
        and (ocr_ready or not ocr_required)
    )
    if json_output:
        print(json.dumps(availability, indent=2))
    else:
        print("\n".join(f"{name}: {'yes' if value else 'no'}" for name, value in availability.items()))
    return 0 if availability["ready"] else 1


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    subparsers = parser.add_subparsers(dest="command", required=True)
    doctor_parser = subparsers.add_parser("doctor")
    doctor_parser.add_argument("--json", action="store_true")
    doctor_parser.add_argument("--require-ocr", action="store_true")
    for name in ("register", "ingest"):
        command = subparsers.add_parser(name)
        command.add_argument("pdf", type=Path)
        command.add_argument("--output", type=Path, default=DEFAULT_OUTPUT)
        command.add_argument("--source-key")
    extract_parser = subparsers.add_parser("extract")
    extract_parser.add_argument("run", type=Path); extract_parser.add_argument("--resume", action="store_true"); extract_parser.add_argument("--columns", type=Path)
    render_parser = subparsers.add_parser("render")
    render_parser.add_argument("run", type=Path); render_parser.add_argument("--pages", default="weak"); render_parser.add_argument("--dpi", type=int, default=300)
    ocr_parser = subparsers.add_parser("ocr")
    ocr_parser.add_argument("run", type=Path); ocr_parser.add_argument("--lang", default="eng"); ocr_parser.add_argument("--psm", type=int, default=6)
    qa_parser = subparsers.add_parser("qa"); qa_parser.add_argument("run", type=Path)
    args = parser.parse_args(argv)
    if args.command == "doctor": return doctor(args.json, args.require_ocr)
    if args.command in {"register", "ingest"}:
        args.output = require_output_path(args.output)
    elif hasattr(args, "run"):
        args.run = require_output_path(args.run)
    if args.command == "register": print(register(args.pdf, args.output, args.source_key)); return 0
    if args.command == "ingest":
        run = register(args.pdf, args.output, args.source_key); extract(run, resume=True)
        if which("pdftoppm"):
            render(run)
            ocr(run)
        else:
            mark_ocr_unavailable(run, "pdftoppm is unavailable")
        status = qa(run); print(run); return status
    if args.command == "extract": extract(args.run, args.resume, args.columns); return 0
    if args.command == "render": render(args.run, args.pages, args.dpi); return 0
    if args.command == "ocr": return ocr(args.run, args.lang, args.psm)
    return qa(args.run)


if __name__ == "__main__":
    raise SystemExit(main())

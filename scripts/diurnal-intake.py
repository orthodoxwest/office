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


def extract(run: Path, resume: bool = False) -> None:
    run_manifest = manifest(run)
    pdf = validate_source(run_manifest)
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
    extract_parser.add_argument("run", type=Path); extract_parser.add_argument("--resume", action="store_true")
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
    if args.command == "extract": extract(args.run, args.resume); return 0
    if args.command == "render": render(args.run, args.pages, args.dpi); return 0
    if args.command == "ocr": return ocr(args.run, args.lang, args.psm)
    return qa(args.run)


if __name__ == "__main__":
    raise SystemExit(main())

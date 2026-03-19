#!/usr/bin/env python3
from __future__ import annotations

import argparse
import random
import string
from pathlib import Path


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Generate 3 coupon code files with both shared(valid) and unique(invalid) codes. Valid coupons are 8-10 characters and appear in at least two files."
    )
    parser.add_argument(
        "count",
        type=int,
        help="Number of coupon lines per file (example: 100, 1000).",
    )
    parser.add_argument(
        "--out-dir",
        type=Path,
        default=Path("generated-coupons"),
        help="Output directory for generated files.",
    )
    parser.add_argument(
        "--valid-percent",
        type=float,
        default=30.0,
        help="Percent of shared(valid) coupons based on count per file (default: 30).",
    )
    parser.add_argument(
        "--seed",
        type=int,
        default=None,
        help="Optional random seed for reproducible output.",
    )
    parser.add_argument(
        "--prefix",
        type=str,
        default="",
        help="Optional prefix included in generated coupon codes. Total code length still stays within 8-10 characters.",
    )
    parser.add_argument(
        "--min-length",
        type=int,
        default=8,
        help="Minimum generated coupon length (default: 8).",
    )
    parser.add_argument(
        "--max-length",
        type=int,
        default=10,
        help="Maximum generated coupon length (default: 10).",
    )
    return parser


def random_code(prefix: str, min_length: int, max_length: int) -> str:
    alphabet = string.ascii_uppercase + string.digits

    if min_length < 8 or max_length > 10 or min_length > max_length:
        raise ValueError("coupon length must stay within 8 to 10 characters")

    if len(prefix) > max_length:
        raise ValueError("prefix length cannot exceed max-length")

    total_length = random.randint(max(min_length, len(prefix)), max_length)
    tail_length = total_length - len(prefix)
    tail = "".join(random.choices(alphabet, k=tail_length))
    return f"{prefix}{tail}"


def unique_code(prefix: str, used: set[str], min_length: int, max_length: int) -> str:
    candidate = random_code(prefix, min_length, max_length)
    while candidate in used:
        candidate = random_code(prefix, min_length, max_length)
    used.add(candidate)
    return candidate


def generate_files(
    count: int,
    out_dir: Path,
    valid_percent: float,
    prefix: str,
    min_length: int,
    max_length: int,
) -> dict[str, int]:
    if count <= 0:
        raise ValueError("count must be greater than 0")
    if not (0.0 <= valid_percent <= 100.0):
        raise ValueError("valid-percent must be between 0 and 100")

    out_dir.mkdir(parents=True, exist_ok=True)

    valid_target = int(round(count * (valid_percent / 100.0)))
    if valid_target == 0 and count >= 2:
        valid_target = 1

    used: set[str] = set()
    valid_codes = [unique_code(prefix, used, min_length, max_length) for _ in range(valid_target)]

    files: list[list[str]] = [[], [], []]
    pairs = [(0, 1), (1, 2), (0, 2)]

    for index, code in enumerate(valid_codes):
        pair = pairs[index % len(pairs)]
        files[pair[0]].append(code)
        files[pair[1]].append(code)

    for file_codes in files:
        while len(file_codes) < count:
            file_codes.append(unique_code(prefix, used, min_length, max_length))
        random.shuffle(file_codes)

    paths = [
        out_dir / "coupons_file_1.txt",
        out_dir / "coupons_file_2.txt",
        out_dir / "coupons_file_3.txt",
    ]

    for path, file_codes in zip(paths, files):
        path.write_text("\n".join(file_codes) + "\n", encoding="utf-8")

    return {
        "count_per_file": count,
        "shared_valid_codes": valid_target,
        "unique_invalid_codes": len(used) - valid_target,
        "total_unique_codes": len(used),
    }


def main() -> None:
    parser = build_parser()
    args = parser.parse_args()

    if args.seed is not None:
        random.seed(args.seed)

    summary = generate_files(
        count=args.count,
        out_dir=args.out_dir,
        valid_percent=args.valid_percent,
        prefix=args.prefix,
        min_length=args.min_length,
        max_length=args.max_length,
    )

    print("Generated files:")
    print(f"- {args.out_dir / 'coupons_file_1.txt'}")
    print(f"- {args.out_dir / 'coupons_file_2.txt'}")
    print(f"- {args.out_dir / 'coupons_file_3.txt'}")
    print("Summary:")
    for key, value in summary.items():
        print(f"- {key}: {value}")


if __name__ == "__main__":
    main()

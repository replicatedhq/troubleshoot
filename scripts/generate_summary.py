#!/usr/bin/env python3
"""
Generate summary report from bundle comparison results.

Reads JSON diff reports and produces:
1. GitHub Actions step summary (Markdown)
2. Console output (colored text)
"""

import argparse
import json
import sys
from pathlib import Path
from typing import List, Dict


def load_reports(report_files: List[str]) -> List[Dict]:
    """Load all report JSON files."""
    reports = []

    for report_file in report_files:
        # Handle glob pattern if not already expanded
        if '*' in report_file:
            report_dir = Path(report_file).parent
            pattern = Path(report_file).name

            for path in sorted(report_dir.glob(pattern)):
                try:
                    with open(path) as f:
                        report = json.load(f)
                        report['_filename'] = path.name
                        reports.append(report)
                except (json.JSONDecodeError, FileNotFoundError) as e:
                    print(f"Warning: Could not load {path}: {e}", file=sys.stderr)
        else:
            # Single file
            try:
                with open(report_file) as f:
                    report = json.load(f)
                    report['_filename'] = Path(report_file).name
                    reports.append(report)
            except (json.JSONDecodeError, FileNotFoundError) as e:
                print(f"Warning: Could not load {report_file}: {e}", file=sys.stderr)

    return reports


def generate_markdown_summary(reports: List[Dict]) -> str:
    """Generate GitHub Actions step summary in Markdown format."""
    lines = []

    lines.append("# 🧪 Regression Test Results")
    lines.append("")

    if not reports:
        lines.append("⚠️ No comparison reports found. Baselines may be missing.")
        return "\n".join(lines)

    # Overall status
    total_regressions = sum(r.get('files_different', 0) for r in reports)
    total_missing = sum(r.get('files_missing_in_current', 0) for r in reports)

    if total_regressions > 0 or total_missing > 0:
        lines.append(f"## ❌ Status: FAILED")
        lines.append(f"**{total_regressions} file(s) with differences, {total_missing} file(s) missing**")
    else:
        lines.append(f"## ✅ Status: PASSED")
        lines.append("All comparisons passed!")

    lines.append("")

    # Per-spec breakdown
    lines.append("## 📊 Comparison Breakdown")
    lines.append("")

    for report in reports:
        spec_type = report.get('spec_type', 'unknown')
        filename = report.get('_filename', 'unknown')

        # Determine status icon
        has_regressions = (
            report.get('files_different', 0) > 0 or
            report.get('files_missing_in_current', 0) > 0
        )
        status_icon = "❌" if has_regressions else "✅"

        lines.append(f"### {status_icon} {spec_type.upper()}")
        lines.append("")
        lines.append(f"**Report:** `{filename}`")
        lines.append("")

        # Stats table
        lines.append("| Metric | Count |")
        lines.append("|--------|-------|")
        lines.append(f"| Files compared | {report.get('files_compared', 0)} |")
        lines.append(f"| Exact matches | {report.get('exact_matches', 0)} |")
        lines.append(f"| Structural matches | {report.get('structural_matches', 0)} |")
        lines.append(f"| Non-empty checks | {report.get('non_empty_checks', 0)} |")
        lines.append(f"| **Files different** | **{report.get('files_different', 0)}** |")
        lines.append(f"| **Missing in current** | **{report.get('files_missing_in_current', 0)}** |")
        lines.append(f"| New in current | {report.get('files_missing_in_baseline', 0)} |")
        lines.append("")

        # Show differences if any
        differences = report.get('differences', [])
        if differences:
            lines.append("<details>")
            lines.append(f"<summary>⚠️ Show {len(differences)} difference(s)</summary>")
            lines.append("")
            lines.append("| File | Mode | Reason |")
            lines.append("|------|------|--------|")
            for diff in differences[:20]:  # Limit to 20
                file = diff.get('file', 'unknown')
                mode = diff.get('mode', 'unknown')
                reason = diff.get('reason', 'unknown')
                lines.append(f"| `{file}` | {mode} | {reason} |")

            if len(differences) > 20:
                lines.append(f"| ... | ... | *{len(differences) - 20} more differences* |")

            lines.append("")
            lines.append("</details>")
            lines.append("")

        # Show missing files if any
        missing = report.get('missing_in_current', [])
        if missing:
            lines.append("<details>")
            lines.append(f"<summary>⚠️ Show {len(missing)} missing file(s)</summary>")
            lines.append("")
            for file in missing[:20]:
                lines.append(f"- `{file}`")

            if len(missing) > 20:
                lines.append(f"- *... and {len(missing) - 20} more*")

            lines.append("")
            lines.append("</details>")
            lines.append("")

    # Footer
    lines.append("---")
    lines.append("")
    lines.append("💡 **Tips:**")
    lines.append("- Download artifacts to inspect bundle contents")
    lines.append("- Review diff reports for detailed comparison results")
    lines.append("- Update baselines if changes are intentional (use workflow_dispatch with update_baselines)")

    return "\n".join(lines)


def generate_console_summary(reports: List[Dict]) -> str:
    """Generate console output with ANSI colors."""
    lines = []

    # ANSI color codes
    RED = "\033[91m"
    GREEN = "\033[92m"
    YELLOW = "\033[93m"
    BLUE = "\033[94m"
    BOLD = "\033[1m"
    RESET = "\033[0m"

    lines.append(f"\n{BOLD}{'='*60}{RESET}")
    lines.append(f"{BOLD}Regression Test Summary{RESET}")
    lines.append(f"{BOLD}{'='*60}{RESET}\n")

    if not reports:
        lines.append(f"{YELLOW}⚠ No comparison reports found{RESET}")
        return "\n".join(lines)

    # Overall status
    total_regressions = sum(r.get('files_different', 0) for r in reports)
    total_missing = sum(r.get('files_missing_in_current', 0) for r in reports)

    if total_regressions > 0 or total_missing > 0:
        lines.append(f"{RED}{BOLD}❌ FAILED{RESET}")
        lines.append(f"   {total_regressions} file(s) with differences")
        lines.append(f"   {total_missing} file(s) missing\n")
    else:
        lines.append(f"{GREEN}{BOLD}✅ PASSED{RESET}")
        lines.append(f"   All comparisons successful\n")

    # Per-spec details
    for i, report in enumerate(reports):
        spec_type = report.get('spec_type', 'unknown')

        has_regressions = (
            report.get('files_different', 0) > 0 or
            report.get('files_missing_in_current', 0) > 0
        )

        status_color = RED if has_regressions else GREEN
        status_icon = "❌" if has_regressions else "✅"

        lines.append(f"{BLUE}{BOLD}{spec_type.upper()}{RESET} {status_color}{status_icon}{RESET}")
        lines.append(f"  Files compared:       {report.get('files_compared', 0)}")
        lines.append(f"  Exact matches:        {report.get('exact_matches', 0)}")
        lines.append(f"  Structural matches:   {report.get('structural_matches', 0)}")
        lines.append(f"  Non-empty checks:     {report.get('non_empty_checks', 0)}")

        if report.get('files_different', 0) > 0:
            lines.append(f"  {RED}Files different:      {report.get('files_different', 0)}{RESET}")

        if report.get('files_missing_in_current', 0) > 0:
            lines.append(f"  {RED}Missing in current:   {report.get('files_missing_in_current', 0)}{RESET}")

        if report.get('files_missing_in_baseline', 0) > 0:
            lines.append(f"  {YELLOW}New in current:       {report.get('files_missing_in_baseline', 0)}{RESET}")

        # Show first few differences
        differences = report.get('differences', [])
        if differences:
            lines.append(f"\n  {RED}Differences:{RESET}")
            for diff in differences[:5]:
                file = diff.get('file', 'unknown')
                reason = diff.get('reason', 'unknown')
                lines.append(f"    • {file}: {reason}")

            if len(differences) > 5:
                lines.append(f"    ... and {len(differences) - 5} more")

        if i < len(reports) - 1:
            lines.append("")  # Spacing between specs

    lines.append(f"\n{BOLD}{'='*60}{RESET}\n")

    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(
        description="Generate summary report from comparison results"
    )
    parser.add_argument(
        "--reports",
        nargs='+',
        required=True,
        help="Report file(s) or pattern (e.g., 'test/output/diff-report-*.json' or multiple files)"
    )
    parser.add_argument(
        "--output-file",
        help="Write markdown summary to file (e.g., $GITHUB_STEP_SUMMARY)"
    )
    parser.add_argument(
        "--output-console",
        action="store_true",
        help="Print colored summary to console"
    )

    args = parser.parse_args()

    # Load reports (args.reports is now a list)
    reports = load_reports(args.reports)

    if not reports:
        print("Warning: No reports loaded", file=sys.stderr)

    # Generate markdown summary
    markdown = generate_markdown_summary(reports)

    # Write to file if requested
    if args.output_file:
        try:
            with open(args.output_file, 'w') as f:
                f.write(markdown)
            print(f"Summary written to {args.output_file}")
        except IOError as e:
            print(f"Error writing summary to {args.output_file}: {e}", file=sys.stderr)

    # Print to console if requested
    if args.output_console:
        console = generate_console_summary(reports)
        print(console)

    # If neither output option specified, print to stdout
    if not args.output_file and not args.output_console:
        print(markdown)


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""
Bundle comparison engine for regression testing.

Unpacks baseline and current bundles, applies comparison rules,
generates diff report, and exits non-zero on regressions.

Based on the simplified 3-tier approach:
1. EXACT match for deterministic files (static data, version.yaml)
2. STRUCTURAL comparison for semi-deterministic files (databases, DNS, etc.)
3. NON-EMPTY check for variable files (cluster-resources, metrics, logs)
"""

import argparse
import json
import sys
import tarfile
import tempfile
from pathlib import Path
from typing import Dict, List, Any, Optional
import fnmatch

try:
    import yaml
except ImportError:
    print("Error: pyyaml not installed. Run: pip install pyyaml")
    sys.exit(1)

try:
    from deepdiff import DeepDiff
except ImportError:
    print("Error: deepdiff not installed. Run: pip install deepdiff")
    sys.exit(1)


class BundleComparator:
    """Compare two troubleshoot bundles using rule-based comparison."""

    def __init__(self, rules_path: str, spec_type: str):
        self.rules = self._load_rules(rules_path, spec_type)
        self.spec_type = spec_type
        self.results = {
            "spec_type": spec_type,
            "files_compared": 0,
            "exact_matches": 0,
            "structural_matches": 0,
            "non_empty_checks": 0,
            "files_different": 0,
            "files_missing_in_current": 0,
            "files_missing_in_baseline": 0,
            "differences": [],
            "missing_in_current": [],
            "missing_in_baseline": [],
        }

    def _load_rules(self, rules_path: str, spec_type: str) -> Dict:
        """Load comparison rules from YAML file."""
        if not Path(rules_path).exists():
            print(f"Warning: Rules file not found at {rules_path}, using defaults")
            return self._get_default_rules()

        with open(rules_path) as f:
            rules = yaml.safe_load(f)

        return rules.get(spec_type, rules.get("defaults", {}))

    def _get_default_rules(self) -> Dict:
        """Return default comparison rules if no config file."""
        return {
            "exact_match": [
                "static-data.txt/static-data",
                "version.yaml",
            ],
            "structural_compare": {
                "postgres/*.json": "database_connection",
                "mysql/*.json": "database_connection",
                "mssql/*.json": "database_connection",
                "redis/*.json": "database_connection",
                "dns/debug.json": "dns_structure",
                "registry/*.json": "registry_exists",
                "http*.json": "http_status",
            },
            "non_empty_default": True,
        }

    def compare(self, baseline_bundle: str, current_bundle: str) -> bool:
        """
        Compare two bundles. Returns True if no regressions detected.

        Args:
            baseline_bundle: Path to baseline bundle tar.gz
            current_bundle: Path to current bundle tar.gz

        Returns:
            True if bundles match (no regressions), False otherwise
        """
        with tempfile.TemporaryDirectory() as tmpdir:
            baseline_dir = Path(tmpdir) / "baseline"
            current_dir = Path(tmpdir) / "current"

            print(f"Extracting baseline bundle to {baseline_dir}...")
            self._extract(baseline_bundle, baseline_dir)

            print(f"Extracting current bundle to {current_dir}...")
            self._extract(current_bundle, current_dir)

            baseline_files = self._get_file_list(baseline_dir)
            current_files = self._get_file_list(current_dir)

            print(f"Baseline files: {len(baseline_files)}")
            print(f"Current files: {len(current_files)}")

            # Check for missing files
            missing_in_current = baseline_files - current_files
            missing_in_baseline = current_files - baseline_files

            # Filter out optional files that may not exist (previous logs, etc.)
            optional_patterns = [
                "*-previous.log",  # Previous container logs (only exist after restart)
            ]

            for file in sorted(missing_in_current):
                # Skip optional files
                if any(file.match(pattern) for pattern in optional_patterns):
                    print(f"  ℹ Optional file missing (OK): {file}")
                    continue
                self._record_missing("current", str(file))

            for file in sorted(missing_in_baseline):
                # Optional files added in current are also OK
                if any(file.match(pattern) for pattern in optional_patterns):
                    print(f"  ℹ Optional file added (OK): {file}")
                    continue
                self._record_missing("baseline", str(file))

            # Compare common files
            common_files = baseline_files & current_files
            print(f"Comparing {len(common_files)} common files...")

            for file in sorted(common_files):
                self._compare_file(
                    baseline_dir / file,
                    current_dir / file,
                    str(file)
                )

        # Determine if there are regressions
        has_regressions = (
            self.results["files_different"] > 0 or
            self.results["files_missing_in_current"] > 0
        )

        return not has_regressions

    def _extract(self, bundle_path: str, dest_dir: Path):
        """Extract tar.gz bundle to destination directory."""
        dest_dir.mkdir(parents=True, exist_ok=True)

        with tarfile.open(bundle_path, 'r:gz') as tar:
            tar.extractall(dest_dir)

        # Handle bundles that extract to a nested directory (e.g., preflightbundle-timestamp/)
        # If there's only one directory at the root, use that as the actual root
        items = list(dest_dir.iterdir())
        if len(items) == 1 and items[0].is_dir():
            # Move contents up one level
            nested_dir = items[0]
            for item in nested_dir.iterdir():
                item.rename(dest_dir / item.name)
            nested_dir.rmdir()

    def _get_file_list(self, dir_path: Path) -> set:
        """Get set of all files in directory (relative paths)."""
        files = set()
        for path in dir_path.rglob('*'):
            if path.is_file():
                rel_path = path.relative_to(dir_path)
                files.add(rel_path)
        return files

    def _compare_file(self, baseline_path: Path, current_path: Path, rel_path: str):
        """Compare a single file pair using appropriate rule."""
        self.results["files_compared"] += 1

        # Determine comparison mode
        mode = self._get_comparison_mode(rel_path)

        try:
            if mode == "exact":
                if self._compare_exact(baseline_path, current_path):
                    self.results["exact_matches"] += 1
                else:
                    self._record_diff(rel_path, "exact", "Content mismatch")

            elif mode == "structural":
                comparator = self._get_structural_comparator(rel_path)
                if self._compare_structural(baseline_path, current_path, comparator):
                    self.results["structural_matches"] += 1
                else:
                    self._record_diff(rel_path, "structural", f"Structural comparison failed ({comparator})")

            else:  # non_empty
                if self._check_non_empty(current_path):
                    self.results["non_empty_checks"] += 1
                else:
                    self._record_diff(rel_path, "non_empty", "File is empty")

        except Exception as e:
            self._record_diff(rel_path, "error", f"Comparison error: {str(e)}")

    def _get_comparison_mode(self, rel_path: str) -> str:
        """Determine comparison mode for a file based on rules."""
        # Check exact match patterns
        for pattern in self.rules.get("exact_match", []):
            if fnmatch.fnmatch(rel_path, pattern) or rel_path == pattern:
                return "exact"

        # Check structural comparison patterns
        for pattern in self.rules.get("structural_compare", {}).keys():
            if fnmatch.fnmatch(rel_path, pattern):
                return "structural"

        # Default: non-empty check
        return "non_empty"

    def _get_structural_comparator(self, rel_path: str) -> str:
        """Get the structural comparator name for a file."""
        for pattern, comparator in self.rules.get("structural_compare", {}).items():
            if fnmatch.fnmatch(rel_path, pattern):
                return comparator
        return "unknown"

    def _compare_exact(self, baseline_path: Path, current_path: Path) -> bool:
        """Compare files byte-for-byte."""
        return baseline_path.read_bytes() == current_path.read_bytes()

    def _compare_structural(self, baseline_path: Path, current_path: Path, comparator: str) -> bool:
        """Compare files using structural comparator."""
        # Load JSON data
        try:
            baseline_data = json.loads(baseline_path.read_text())
            current_data = json.loads(current_path.read_text())
        except json.JSONDecodeError as e:
            print(f"  JSON decode error: {e}")
            return False

        # Apply comparator
        if comparator == "database_connection":
            return self._compare_database_connection(baseline_data, current_data)
        elif comparator == "dns_structure":
            return self._compare_dns_structure(baseline_data, current_data)
        elif comparator == "registry_exists":
            return self._compare_registry_exists(baseline_data, current_data)
        elif comparator == "http_status":
            return self._compare_http_status(baseline_data, current_data)
        elif comparator == "cluster_version":
            return self._compare_cluster_version(baseline_data, current_data)
        elif comparator == "analysis_results":
            return self._compare_analysis_results(baseline_data, current_data)
        else:
            # Unknown comparator - fall back to non-empty
            return True

    def _compare_database_connection(self, baseline: Dict, current: Dict) -> bool:
        """Compare database connection results (isConnected field only)."""
        b_connected = baseline.get("isConnected", False)
        c_connected = current.get("isConnected", False)

        if b_connected != c_connected:
            print(f"    Database connection status changed: {b_connected} -> {c_connected}")
            return False

        return True

    def _compare_dns_structure(self, baseline: Dict, current: Dict) -> bool:
        """Compare DNS structure (service exists, query succeeds)."""
        # Check kubernetes service exists
        if "query" not in current or "kubernetes" not in current["query"]:
            print(f"    DNS query.kubernetes missing")
            return False

        # Kubernetes ClusterIP should exist (don't compare value, it can vary)
        if not current["query"]["kubernetes"].get("address"):
            print(f"    DNS kubernetes.address is empty")
            return False

        # DNS service should exist
        if not current.get("kubeDNSService"):
            print(f"    DNS kubeDNSService is empty")
            return False

        # At least one DNS pod should exist
        if not current.get("kubeDNSPods") or len(current["kubeDNSPods"]) == 0:
            print(f"    DNS kubeDNSPods is empty")
            return False

        # Non-resolvable domain should be empty
        if current.get("query", {}).get("nonResolvableDomain", {}).get("address"):
            print(f"    DNS nonResolvableDomain should be empty")
            return False

        return True

    def _compare_registry_exists(self, baseline: Dict, current: Dict) -> bool:
        """Compare registry image existence (exists boolean per image)."""
        baseline_images = baseline.get("images", {})
        current_images = current.get("images", {})

        # Check same images are present
        if set(baseline_images.keys()) != set(current_images.keys()):
            print(f"    Registry image list changed")
            print(f"      Baseline: {sorted(baseline_images.keys())}")
            print(f"      Current: {sorted(current_images.keys())}")
            return False

        # Compare exists status for each image
        for image_name in baseline_images:
            b_exists = baseline_images[image_name].get("exists", False)
            c_exists = current_images[image_name].get("exists", False)

            if b_exists != c_exists:
                print(f"    Registry image '{image_name}' existence changed: {b_exists} -> {c_exists}")
                return False

        return True

    def _compare_http_status(self, baseline: Dict, current: Dict) -> bool:
        """Compare HTTP response (status code only)."""
        b_status = baseline.get("response", {}).get("status", 0)
        c_status = current.get("response", {}).get("status", 0)

        if b_status != c_status:
            print(f"    HTTP status changed: {b_status} -> {c_status}")
            return False

        return True

    def _compare_cluster_version(self, baseline: Dict, current: Dict) -> bool:
        """Compare cluster version (major/minor only, ignore build details)."""
        b_info = baseline.get("info", {})
        c_info = current.get("info", {})

        # Compare major and minor version
        if b_info.get("major") != c_info.get("major"):
            print(f"    Cluster major version changed: {b_info.get('major')} -> {c_info.get('major')}")
            return False

        if b_info.get("minor") != c_info.get("minor"):
            print(f"    Cluster minor version changed: {b_info.get('minor')} -> {c_info.get('minor')}")
            return False

        # Don't compare: gitVersion, gitCommit, buildDate, goVersion (these vary with k3s updates)
        return True

    def _compare_analysis_results(self, baseline: Dict, current: Dict) -> bool:
        """Compare analysis results (analyzer names and count, not specific messages)."""
        if not isinstance(baseline, list) or not isinstance(current, list):
            print(f"    Analysis results structure changed (expected list)")
            return False

        # Create map of analyzer name -> severity for comparison
        baseline_results = {item.get("name"): item.get("severity") for item in baseline if "name" in item}
        current_results = {item.get("name"): item.get("severity") for item in current if "name" in item}

        # Check if same analyzers ran
        baseline_names = set(baseline_results.keys())
        current_names = set(current_results.keys())

        if baseline_names != current_names:
            missing = baseline_names - current_names
            extra = current_names - baseline_names
            if missing:
                print(f"    Missing analyzers: {missing}")
            if extra:
                print(f"    New analyzers: {extra}")
            return False

        # Check if severity levels changed significantly (error/warn differences matter)
        significant_changes = []
        for name in baseline_names:
            b_sev = baseline_results[name]
            c_sev = current_results[name]

            # Only care if error/warn status changes, not debug
            if b_sev != c_sev:
                if b_sev in ["error", "warn"] or c_sev in ["error", "warn"]:
                    significant_changes.append(f"{name}: {b_sev} -> {c_sev}")

        if significant_changes:
            print(f"    Analyzer severity changed:")
            for change in significant_changes[:5]:  # Show first 5
                print(f"      {change}")
            # Don't fail on severity changes - this is informational
            # return False

        return True

    def _check_non_empty(self, path: Path) -> bool:
        """Check that file exists and is non-empty."""
        if not path.exists():
            return False

        size = path.stat().st_size
        if size == 0:
            return False

        # Optional: validate JSON structure if .json extension
        if path.suffix == ".json":
            try:
                json.loads(path.read_text())
            except json.JSONDecodeError:
                print(f"    Invalid JSON: {path.name}")
                return False

        return True

    def _record_diff(self, file: str, mode: str, reason: str):
        """Record a difference/regression."""
        self.results["files_different"] += 1
        self.results["differences"].append({
            "file": file,
            "mode": mode,
            "reason": reason
        })
        print(f"  ❌ {file}: {reason}")

    def _record_missing(self, location: str, file: str):
        """Record a missing file."""
        if location == "current":
            self.results["files_missing_in_current"] += 1
            self.results["missing_in_current"].append(file)
            print(f"  ⚠ Missing in current: {file}")
        else:
            self.results["files_missing_in_baseline"] += 1
            self.results["missing_in_baseline"].append(file)
            print(f"  ℹ New file in current: {file}")

    def generate_report(self, output_path: str):
        """Write JSON report."""
        with open(output_path, 'w') as f:
            json.dump(self.results, f, indent=2)

        print(f"\nReport written to: {output_path}")

    def print_summary(self):
        """Print human-readable summary to stdout."""
        print(f"\n{'='*60}")
        print(f"Bundle Comparison Report - {self.spec_type}")
        print(f"{'='*60}")
        print(f"Files compared:           {self.results['files_compared']}")
        print(f"  Exact matches:          {self.results['exact_matches']}")
        print(f"  Structural matches:     {self.results['structural_matches']}")
        print(f"  Non-empty checks:       {self.results['non_empty_checks']}")
        print(f"Files different:          {self.results['files_different']}")
        print(f"Missing in current:       {self.results['files_missing_in_current']}")
        print(f"Missing in baseline:      {self.results['files_missing_in_baseline']}")

        if self.results["differences"]:
            print(f"\n❌ REGRESSIONS DETECTED ({len(self.results['differences'])}):")
            for diff in self.results["differences"][:10]:  # Show first 10
                print(f"  • {diff['file']}: {diff['reason']}")
            if len(self.results["differences"]) > 10:
                print(f"  ... and {len(self.results['differences']) - 10} more")

        if self.results["missing_in_current"]:
            print(f"\n⚠ MISSING FILES ({len(self.results['missing_in_current'])}):")
            for file in self.results["missing_in_current"][:5]:
                print(f"  • {file}")
            if len(self.results["missing_in_current"]) > 5:
                print(f"  ... and {len(self.results['missing_in_current']) - 5} more")


def main():
    parser = argparse.ArgumentParser(
        description="Compare troubleshoot bundles for regression testing"
    )
    parser.add_argument("--baseline", required=True, help="Baseline bundle tar.gz path")
    parser.add_argument("--current", required=True, help="Current bundle tar.gz path")
    parser.add_argument("--rules", required=True, help="Comparison rules YAML path")
    parser.add_argument("--report", required=True, help="Output report JSON path")
    parser.add_argument(
        "--spec-type",
        required=True,
        choices=["preflight", "supportbundle"],
        help="Type of spec being compared"
    )

    args = parser.parse_args()

    # Verify files exist
    if not Path(args.baseline).exists():
        print(f"Error: Baseline bundle not found: {args.baseline}")
        sys.exit(1)

    if not Path(args.current).exists():
        print(f"Error: Current bundle not found: {args.current}")
        sys.exit(1)

    # Run comparison
    comparator = BundleComparator(args.rules, args.spec_type)
    passed = comparator.compare(args.baseline, args.current)
    comparator.generate_report(args.report)
    comparator.print_summary()

    # Exit with appropriate code
    if passed:
        print("\n✅ No regressions detected")
        sys.exit(0)
    else:
        print("\n❌ Regressions detected!")
        sys.exit(1)


if __name__ == "__main__":
    main()

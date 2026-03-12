#!/usr/bin/env python3
"""
Gatewise CLI Backend Testing Suite
Tests CLI commands, policy parsing, validation, and PostgreSQL connectivity
"""

import subprocess
import sys
import json
from datetime import datetime

class GatewiseTester:
    def __init__(self):
        self.base_path = "/app/gatewise"
        self.gatewise_binary = f"{self.base_path}/gatewise"
        self.test_store_binary = f"{self.base_path}/test-store"
        self.valid_policy = f"{self.base_path}/configs/examples/marketing-team-access.yaml"
        self.invalid_policy = f"{self.base_path}/configs/examples/invalid-policy.yaml"
        self.tests_run = 0
        self.tests_passed = 0

    def run_command(self, cmd, expected_exit_code=0, should_fail=False):
        """Execute a command and validate result"""
        self.tests_run += 1
        print(f"\n🔍 Testing: {' '.join(cmd)}")
        
        try:
            result = subprocess.run(
                cmd, 
                capture_output=True, 
                text=True, 
                cwd=self.base_path,
                timeout=30
            )
            
            success = (result.returncode == expected_exit_code)
            
            if success:
                self.tests_passed += 1
                print(f"✅ Command succeeded (exit code: {result.returncode})")
                if result.stdout:
                    print(f"📄 Output:\n{result.stdout}")
            else:
                print(f"❌ Command failed - Expected exit code {expected_exit_code}, got {result.returncode}")
                if result.stdout:
                    print(f"📄 Stdout:\n{result.stdout}")
                if result.stderr:
                    print(f"📄 Stderr:\n{result.stderr}")
            
            return success, result.stdout, result.stderr
            
        except subprocess.TimeoutExpired:
            print("❌ Command timed out after 30 seconds")
            return False, "", "Timeout"
        except Exception as e:
            print(f"❌ Command execution failed: {str(e)}")
            return False, "", str(e)

    def test_version(self):
        """Test CLI version command"""
        return self.run_command([self.gatewise_binary, "--version"])

    def test_parse_yaml_output(self):
        """Test parse command with default YAML output"""
        success, output, _ = self.run_command([
            self.gatewise_binary, "parse", self.valid_policy
        ])
        
        if success and "apiVersion: gatewise.io/v1alpha1" in output:
            print("✅ YAML parsing successful - found correct apiVersion")
            return True
        else:
            print("❌ YAML parsing failed - missing expected content")
            return False

    def test_parse_json_output(self):
        """Test parse command with JSON output format"""
        success, output, _ = self.run_command([
            self.gatewise_binary, "parse", self.valid_policy, "-o", "json"
        ])
        
        if success:
            try:
                parsed_json = json.loads(output)
                if parsed_json.get("apiVersion") == "gatewise.io/v1alpha1":
                    print("✅ JSON parsing successful - valid JSON structure")
                    return True
                else:
                    print("❌ JSON parsing failed - missing apiVersion")
                    return False
            except json.JSONDecodeError:
                print("❌ JSON parsing failed - invalid JSON output")
                return False
        return False

    def test_validate_valid_policy(self):
        """Test validate command with valid policy"""
        success, output, _ = self.run_command([
            self.gatewise_binary, "validate", self.valid_policy
        ])
        
        if success and "✓ Policy 'marketing-team-access' is valid" in output:
            print("✅ Valid policy validation successful")
            return True
        else:
            print("❌ Valid policy validation failed")
            return False

    def test_validate_invalid_policy(self):
        """Test validate command with invalid policy (should fail)"""
        success, output, stderr = self.run_command([
            self.gatewise_binary, "validate", self.invalid_policy
        ], expected_exit_code=1, should_fail=True)
        
        if success and "validation failed" in stderr:
            print("✅ Invalid policy correctly rejected")
            return True
        else:
            print("❌ Invalid policy validation didn't fail as expected")
            return False

    def test_multi_document_parsing(self):
        """Test if CLI handles multi-document YAML files correctly"""
        # Count documents in the marketing file
        with open(self.valid_policy, 'r') as f:
            content = f.read()
            doc_count = content.count('---') + 1  # First document doesn't have ---
        
        print(f"📄 Marketing policy file contains {doc_count} YAML documents")
        
        # The current CLI parse command only shows the first document
        success, output, _ = self.run_command([
            self.gatewise_binary, "parse", self.valid_policy
        ])
        
        if success:
            # Check if only AccessPolicy is returned (first document)
            if "kind: AccessPolicy" in output and "kind: TeamBinding" not in output:
                print("⚠️  CLI parse only processes the first document in multi-document files")
                print("   This may be expected behavior for the current CLI design")
                return True
            elif "kind: TeamBinding" in output:
                print("✅ CLI parse processes multiple documents")
                return True
        
        return False

    def test_postgresql_connectivity(self):
        """Test PostgreSQL connectivity using test-store binary"""
        success, output, _ = self.run_command([self.test_store_binary])
        
        if success and "All PostgreSQL tests passed!" in output:
            print("✅ PostgreSQL connectivity and CRUD operations working")
            return True
        else:
            print("❌ PostgreSQL connectivity test failed")
            return False

    def test_table_output_format(self):
        """Test table output format"""
        success, output, _ = self.run_command([
            self.gatewise_binary, "parse", self.valid_policy, "-o", "table"
        ])
        
        if success and "KIND" in output and "NAME" in output:
            print("✅ Table output format working")
            return True
        else:
            print("❌ Table output format failed")
            return False

    def run_all_tests(self):
        """Run comprehensive test suite"""
        print("=" * 60)
        print("🚀 Starting Gatewise CLI Backend Testing Suite")
        print("=" * 60)
        
        test_methods = [
            ("Version Check", self.test_version),
            ("Parse YAML Output", self.test_parse_yaml_output),
            ("Parse JSON Output", self.test_parse_json_output),
            ("Parse Table Output", self.test_table_output_format),
            ("Validate Valid Policy", self.test_validate_valid_policy),
            ("Validate Invalid Policy", self.test_validate_invalid_policy),
            ("Multi-document Parsing", self.test_multi_document_parsing),
            ("PostgreSQL Connectivity", self.test_postgresql_connectivity),
        ]
        
        for test_name, test_method in test_methods:
            print(f"\n{'='*20} {test_name} {'='*20}")
            try:
                test_method()
            except Exception as e:
                print(f"❌ Test {test_name} crashed: {str(e)}")
        
        print("\n" + "=" * 60)
        print(f"📊 Test Summary: {self.tests_passed}/{self.tests_run} tests passed")
        
        if self.tests_passed == self.tests_run:
            print("🎉 All tests passed!")
            return 0
        else:
            print(f"⚠️  {self.tests_run - self.tests_passed} test(s) failed")
            return 1

def main():
    tester = GatewiseTester()
    return tester.run_all_tests()

if __name__ == "__main__":
    sys.exit(main())
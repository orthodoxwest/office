#!/usr/bin/env python3
"""Tests for scripts/appstore.py: its choice of version and build, and the requests it makes,
with App Store Connect stood in for by responses shaped like the API's."""

import contextlib
import importlib.util
import io
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

SCRIPT = pathlib.Path(__file__).with_name("appstore.py")
SPEC = importlib.util.spec_from_file_location("appstore", SCRIPT)
STORE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = STORE
SPEC.loader.exec_module(STORE)

dt = STORE.dt
NOW = dt.datetime(2026, 11, 16, 15, 0, tzinfo=dt.timezone.utc)
APP = "6739000000"


def version(vid, string, app_state, store_state=None, release="AFTER_APPROVAL"):
    """An appStoreVersions resource as GET /v1/apps/{id}/appStoreVersions returns it."""
    return {
        "type": "appStoreVersions",
        "id": vid,
        "attributes": {
            "platform": "IOS",
            "versionString": string,
            "appStoreState": store_state or app_state,
            "appVersionState": app_state,
            "copyright": None,
            "releaseType": release,
            "earliestReleaseDate": None,
            "downloadable": True,
            "createdDate": "2026-11-01T10:00:00-07:00",
        },
        "links": {"self": f"https://api.appstoreconnect.apple.com/v1/appStoreVersions/{vid}"},
    }


def prerelease(string):
    return {"type": "preReleaseVersions", "id": f"pre-{string}",
            "attributes": {"version": string, "platform": "IOS"}}


def submission(sid, state):
    return {"type": "reviewSubmissions", "id": sid,
            "attributes": {"platform": "IOS", "submittedDate": None, "state": state}}


def builds_response(*builds):
    """GET /v1/builds?include=preReleaseVersion,buildBetaDetail for (number, version, hours ago,
    processingState, externalBuildState[, usesNonExemptEncryption]) tuples."""
    data, included = [], []
    for spec in builds:
        number, string, hours, processing, external, *rest = spec
        uploaded = (NOW - dt.timedelta(hours=hours)).astimezone(dt.timezone(dt.timedelta(hours=-8)))
        data.append({
            "type": "builds",
            "id": f"build-{number}",
            "attributes": {
                "version": str(number),
                "uploadedDate": uploaded.isoformat(),
                "expirationDate": (uploaded + dt.timedelta(days=90)).isoformat(),
                "expired": False,
                "minOsVersion": "17.0",
                "processingState": processing,
                "buildAudienceType": "APP_STORE_ELIGIBLE",
                "usesNonExemptEncryption": rest[0] if rest else False,
            },
            "relationships": {
                "preReleaseVersion": {"data": {"type": "preReleaseVersions", "id": f"pre-{string}"}},
                "buildBetaDetail": {"data": {"type": "buildBetaDetails", "id": f"beta-{number}"}},
            },
        })
        included.append({"type": "buildBetaDetails", "id": f"beta-{number}", "attributes": {
            "autoNotifyEnabled": True, "internalBuildState": "IN_BETA_TESTING", "externalBuildState": external}})
        if not any(i["id"] == f"pre-{string}" for i in included):
            included.append(prerelease(string))
    return {"data": data, "included": included, "links": {"self": "https://api.appstoreconnect.apple.com/v1/builds"},
            "meta": {"paging": {"total": len(data), "limit": 200}}}


ON_SALE = version("v100", "1.0.0", "READY_FOR_DISTRIBUTION", "READY_FOR_SALE")


class VersionTest(unittest.TestCase):
    def test_project_yml_is_the_floor(self):
        self.assertEqual(STORE.next_marketing_version("1.0.0", [], []), "1.0.0")
        self.assertEqual(STORE.next_marketing_version("1.1.0", [ON_SALE], [prerelease("1.0.1")]), "1.1.0")

    def test_a_version_in_review_or_on_sale_moves_uploads_on_a_patch(self):
        for app_state in ["WAITING_FOR_REVIEW", "IN_REVIEW", "PENDING_DEVELOPER_RELEASE", "READY_FOR_DISTRIBUTION",
                          "PROCESSING_FOR_DISTRIBUTION", "ACCEPTED", "REPLACED_WITH_NEW_VERSION"]:
            with self.subTest(app_state):
                self.assertEqual(STORE.next_marketing_version("1.0.0", [version("v", "1.0.0", app_state)],
                                                              [prerelease("1.0.0")]), "1.0.1")

    def test_the_older_state_field_alone_is_read(self):
        v = version("v", "1.0.0", "READY_FOR_SALE")
        del v["attributes"]["appVersionState"]
        self.assertEqual(STORE.next_marketing_version("1.0.0", [v], []), "1.0.1")

    def test_a_version_being_prepared_or_rejected_takes_uploads(self):
        for app_state in ["PREPARE_FOR_SUBMISSION", "DEVELOPER_REJECTED", "REJECTED", "METADATA_REJECTED"]:
            with self.subTest(app_state):
                self.assertEqual(STORE.next_marketing_version("1.0.0", [version("v", "1.0.0", app_state)], []), "1.0.0")

    def test_one_patch_above_the_highest_closed_version(self):
        versions = [ON_SALE, version("v101", "1.0.1", "READY_FOR_DISTRIBUTION"),
                    version("v102", "1.0.2", "WAITING_FOR_REVIEW")]
        self.assertEqual(STORE.next_marketing_version("1.0.0", versions, []), "1.0.3")
        self.assertEqual(STORE.next_marketing_version("1.0", [version("v", "1.0", "READY_FOR_SALE")], []), "1.0.1")

    def test_never_below_a_version_testflight_already_has(self):
        # 1.0.1 was submitted and rejected after builds had moved on to 1.0.2.
        versions = [ON_SALE, version("v101", "1.0.1", "REJECTED")]
        self.assertEqual(STORE.next_marketing_version("1.0.0", versions, [prerelease("1.0.1"), prerelease("1.0.2")]),
                         "1.0.2")

    def test_floor_is_read_from_project_yml(self):
        self.assertRegex(STORE.floor_version(), r"^\d+\.\d+(\.\d+)?$")


class DecideTest(unittest.TestCase):
    def decide(self, versions, builds, submissions=(), min_age=24, number=0):
        return STORE.decide(list(versions), list(submissions), STORE.builds_of(builds), NOW, min_age, number)

    def test_the_newest_tested_build_a_day_old_is_chosen(self):
        builds = builds_response((830, "1.0.1", 2, "VALID", "IN_BETA_TESTING"),
                                 (825, "1.0.1", 30, "VALID", "IN_BETA_TESTING"),
                                 (820, "1.0.1", 50, "VALID", "IN_BETA_TESTING"),
                                 (810, "1.0.0", 200, "VALID", "IN_BETA_TESTING"))
        plan, _ = self.decide([ON_SALE], builds)
        self.assertEqual((plan["build"]["number"], plan["build"]["version"]), (825, "1.0.1"))
        self.assertIsNone(plan["reuse"])
        self.assertIsNone(plan["draft"])
        self.assertTrue(plan["update"])

    def test_only_builds_with_the_external_testers(self):
        for external in ["WAITING_FOR_BETA_REVIEW", "IN_BETA_REVIEW", "READY_FOR_BETA_SUBMISSION", "BETA_REJECTED",
                         "EXPIRED", "MISSING_EXPORT_COMPLIANCE"]:
            with self.subTest(external):
                plan, why = self.decide([ON_SALE], builds_response((825, "1.0.1", 30, "VALID", external)))
                self.assertIsNone(plan)
                self.assertIn("Nothing to submit", why)

    def test_only_valid_builds_above_the_live_version(self):
        builds = builds_response((830, "1.0.1", 30, "PROCESSING", "IN_BETA_TESTING"),
                                 (829, "1.0.1", 30, "INVALID", "IN_BETA_TESTING"),
                                 (800, "1.0.0", 300, "VALID", "IN_BETA_TESTING"))
        self.assertIsNone(self.decide([ON_SALE], builds)[0])

    def test_nothing_while_a_version_is_with_apple_or_waiting_to_go_out(self):
        builds = builds_response((825, "1.0.1", 30, "VALID", "IN_BETA_TESTING"))
        for app_state in ["WAITING_FOR_REVIEW", "IN_REVIEW", "PENDING_DEVELOPER_RELEASE", "PROCESSING_FOR_DISTRIBUTION"]:
            with self.subTest(app_state):
                plan, why = self.decide([ON_SALE, version("v101", "1.0.1", app_state)], builds)
                self.assertIsNone(plan)
                self.assertIn("1.0.1", why)

    def test_a_rejection_waits_for_a_person(self):
        builds = builds_response((830, "1.0.2", 30, "VALID", "IN_BETA_TESTING"))
        plan, why = self.decide([ON_SALE, version("v101", "1.0.1", "REJECTED")], builds)
        self.assertIsNone(plan)
        self.assertIn("rejected", why)

    def test_an_open_review_submission_waits(self):
        builds = builds_response((825, "1.0.1", 30, "VALID", "IN_BETA_TESTING"))
        plan, why = self.decide([ON_SALE], builds, [submission("s1", "UNRESOLVED_ISSUES")])
        self.assertIsNone(plan)
        self.assertIn("unresolved issues", why)

    def test_a_version_being_prepared_and_a_draft_submission_are_reused(self):
        builds = builds_response((825, "1.0.1", 30, "VALID", "IN_BETA_TESTING"))
        prepared = version("v101", "1.0.1", "PREPARE_FOR_SUBMISSION", release="MANUAL")
        draft = submission("s1", "READY_FOR_REVIEW")
        plan, _ = self.decide([ON_SALE, prepared], builds, [draft])
        self.assertIs(plan["reuse"], prepared)
        self.assertIs(plan["draft"], draft)

    def test_a_higher_version_prepared_by_hand_is_left_alone(self):
        builds = builds_response((825, "1.0.1", 30, "VALID", "IN_BETA_TESTING"))
        plan, why = self.decide([ON_SALE, version("v110", "1.1.0", "PREPARE_FOR_SUBMISSION")], builds)
        self.assertIsNone(plan)
        self.assertIn("1.1.0", why)

    def test_manual_takes_any_age_or_the_build_named(self):
        builds = builds_response((830, "1.0.1", 2, "VALID", "IN_BETA_TESTING"),
                                 (828, "1.0.1", 1, "VALID", "IN_BETA_REVIEW"))
        self.assertEqual(self.decide([ON_SALE], builds, min_age=0)[0]["build"]["number"], 830)
        self.assertEqual(self.decide([ON_SALE], builds, min_age=0, number=828)[0]["build"]["number"], 828)
        self.assertIsNone(self.decide([ON_SALE], builds, min_age=0, number=999)[0])

    def test_the_first_release_has_no_update_steps(self):
        plan, _ = self.decide([], builds_response((800, "1.0.0", 30, "VALID", "IN_BETA_TESTING")))
        self.assertFalse(plan["update"])


class FakeApi:
    """Records writes; answers reads from a table of path prefixes."""

    def __init__(self, reads):
        self.reads, self.writes = reads, []

    def call(self, method, path, body=None):
        if method == "GET":
            return next(v for k, v in self.reads.items() if path.startswith(k))
        self.writes.append((method, path, body))
        if method == "POST":
            return {"data": {"type": body["data"]["type"], "id": f"new-{body['data']['type']}"}}
        return {}

    def get_all(self, path):
        return self.call("GET", path)


LOCALIZATION = {"type": "appStoreVersionLocalizations", "id": "loc-en",
                "attributes": {"locale": "en-US", "whatsNew": None, "description": "The Divine Office."}}


class SubmitTest(unittest.TestCase):
    def plan(self, versions, submissions=(), encryption=False):
        builds = builds_response((825, "1.0.1", 30, "VALID", "IN_BETA_TESTING", encryption))
        plan, why = STORE.decide(versions, list(submissions), STORE.builds_of(builds), NOW, 24)
        self.assertIsNotNone(plan, why)
        return plan

    def test_a_new_version_is_created_filled_in_and_submitted(self):
        api = FakeApi({"/v1/appStoreVersions/new-appStoreVersions/appStoreVersionLocalizations": {"data": [LOCALIZATION]},
                       "/v1/appStoreVersions/new-appStoreVersions/appStoreVersionPhasedRelease": {"data": None}})
        STORE.submit(api, APP, self.plan([ON_SALE]), "Feast pages redesigned.")
        writes = [(m, p) for m, p, _ in api.writes]
        self.assertEqual(writes, [
            ("POST", "/v1/appStoreVersions"),
            ("PATCH", "/v1/appStoreVersionLocalizations/loc-en"),
            ("POST", "/v1/appStoreVersionPhasedReleases"),
            ("POST", "/v1/reviewSubmissions"),
            ("POST", "/v1/reviewSubmissionItems"),
            ("PATCH", "/v1/reviewSubmissions/new-reviewSubmissions"),
        ])
        create = api.writes[0][2]["data"]
        self.assertEqual(create["attributes"], {"platform": "IOS", "versionString": "1.0.1", "releaseType": "AFTER_APPROVAL"})
        self.assertEqual(create["relationships"]["build"]["data"], {"type": "builds", "id": "build-825"})
        self.assertEqual(api.writes[1][2]["data"]["attributes"], {"whatsNew": "Feast pages redesigned."})
        self.assertEqual(api.writes[2][2]["data"]["relationships"]["appStoreVersion"]["data"]["id"], "new-appStoreVersions")
        item = api.writes[4][2]["data"]["relationships"]
        self.assertEqual(item["reviewSubmission"]["data"]["id"], "new-reviewSubmissions")
        self.assertEqual(item["appStoreVersion"]["data"]["id"], "new-appStoreVersions")
        self.assertEqual(api.writes[5][2]["data"]["attributes"], {"submitted": True})

    def test_a_rerun_reuses_what_the_first_run_made(self):
        prepared = version("v101", "1.0.1", "PREPARE_FOR_SUBMISSION")
        draft = submission("s1", "READY_FOR_REVIEW")
        item = {"type": "reviewSubmissionItems", "id": "i1", "attributes": {"state": "READY_FOR_REVIEW"},
                "relationships": {"appStoreVersion": {"data": {"type": "appStoreVersions", "id": "v101"}}}}
        done = dict(LOCALIZATION, attributes=dict(LOCALIZATION["attributes"], whatsNew="Notes."))
        api = FakeApi({"/v1/appStoreVersions/v101/appStoreVersionLocalizations": {"data": [done]},
                       "/v1/appStoreVersions/v101/appStoreVersionPhasedRelease": {"data": {
                           "type": "appStoreVersionPhasedReleases", "id": "p1",
                           "attributes": {"phasedReleaseState": "INACTIVE"}}},
                       "/v1/reviewSubmissions/s1/items": {"data": [item]}})
        STORE.submit(api, APP, self.plan([ON_SALE, prepared], [draft]), "Notes.")
        self.assertEqual([(m, p) for m, p, _ in api.writes], [
            ("PATCH", "/v1/appStoreVersions/v101/relationships/build"),
            ("PATCH", "/v1/reviewSubmissions/s1"),
        ])
        self.assertEqual(api.writes[0][2], {"data": {"type": "builds", "id": "build-825"}})

    def test_a_prepared_version_is_renumbered_and_set_to_release_on_approval(self):
        prepared = version("v101", "1.0.0", "DEVELOPER_REJECTED", release="MANUAL")
        api = FakeApi({"/v1/appStoreVersions/v101/appStoreVersionLocalizations": {"data": []},
                       "/v1/appStoreVersions/v101/appStoreVersionPhasedRelease": {"data": None},
                       "/v1/reviewSubmissions": {"data": []}})
        plan = self.plan([prepared])
        STORE.submit(api, APP, plan, "Notes.")
        self.assertEqual(api.writes[0][:2], ("PATCH", "/v1/appStoreVersions/v101"))
        self.assertEqual(api.writes[0][2]["data"]["attributes"], {"versionString": "1.0.1", "releaseType": "AFTER_APPROVAL"})

    def test_export_compliance_is_answered_only_when_missing(self):
        api = FakeApi({"/v1/appStoreVersions/new-appStoreVersions/appStoreVersionLocalizations": {"data": []},
                       "/v1/appStoreVersions/new-appStoreVersions/appStoreVersionPhasedRelease": {"data": None}})
        STORE.submit(api, APP, self.plan([ON_SALE], encryption=None), "Notes.")
        patch = [b for m, p, b in api.writes if p == "/v1/builds/build-825"]
        self.assertEqual(patch[0]["data"]["attributes"], {"usesNonExemptEncryption": False})


class MainTest(unittest.TestCase):
    def run_main(self, argv, reads):
        api = FakeApi(reads)
        out, err = io.StringIO(), io.StringIO()
        with mock.patch.object(STORE, "Api", lambda *a, **k: api), \
                mock.patch.object(STORE, "now", lambda: NOW), \
                mock.patch.object(STORE, "commit_of", lambda n: None), \
                mock.patch.object(STORE, "notes_at", lambda c: ""), \
                mock.patch.dict(os.environ, {"ASC_KEY_ID": "K", "ASC_ISSUER_ID": "I", "ASC_KEY": "P",
                                             "GITHUB_STEP_SUMMARY": ""}), \
                mock.patch.object(sys, "argv", ["appstore.py", *argv]), \
                contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            STORE.main()
        return api, out.getvalue(), err.getvalue()

    READS = {
        "/v1/apps?": {"data": [{"type": "apps", "id": APP, "attributes": {"bundleId": "org.orthodoxwest.office"}}]},
        f"/v1/apps/{APP}/appStoreVersions": {"data": [version("v100", "1.0.0", "WAITING_FOR_REVIEW")]},
        "/v1/preReleaseVersions": {"data": [prerelease("1.0.0")]},
        "/v1/reviewSubmissions": {"data": [submission("s0", "WAITING_FOR_REVIEW")]},
        "/v1/builds": builds_response((825, "1.0.0", 30, "VALID", "IN_BETA_TESTING")),
    }

    def test_version_prints_the_next_version(self):
        api, out, _ = self.run_main(["version"], self.READS)
        expected = STORE.next_marketing_version(STORE.floor_version(), self.READS[f"/v1/apps/{APP}/appStoreVersions"]["data"],
                                                [prerelease("1.0.0")])
        self.assertEqual(out.strip(), expected)
        self.assertEqual(api.writes, [])

    def test_dry_run_writes_nothing(self):
        reads = dict(self.READS)
        reads[f"/v1/apps/{APP}/appStoreVersions"] = {"data": [ON_SALE]}
        reads["/v1/reviewSubmissions"] = {"data": []}
        reads["/v1/builds"] = builds_response((825, "1.0.1", 30, "VALID", "IN_BETA_TESTING"))
        api, _, err = self.run_main(["manual", "--dry-run"], reads)
        self.assertEqual(api.writes, [])
        self.assertIn("Dry run: Submitting build 825 as version 1.0.1", err)
        self.assertIn(STORE.DEFAULT_NOTES, err)


class TokenTest(unittest.TestCase):
    def test_der_signature_to_raw(self):
        r, s = b"\x00\x81" + b"\x01" * 31, b"\x05" * 20
        der = bytes([0x30, 4 + len(r) + len(s), 0x02, len(r)]) + r + bytes([0x02, len(s)]) + s
        raw = STORE.der_to_raw(der)
        self.assertEqual(raw, b"\x81" + b"\x01" * 31 + b"\x00" * 12 + b"\x05" * 20)

    @unittest.skipUnless(shutil.which("openssl"), "needs openssl")
    def test_token_verifies_with_the_public_key(self):
        with tempfile.TemporaryDirectory() as tmp:
            key, pub = pathlib.Path(tmp, "k.p8"), pathlib.Path(tmp, "k.pub")
            subprocess.run(["openssl", "genpkey", "-algorithm", "EC", "-pkeyopt", "ec_paramgen_curve:P-256",
                            "-out", str(key)], check=True, capture_output=True)
            subprocess.run(["openssl", "pkey", "-in", str(key), "-pubout", "-out", str(pub)], check=True, capture_output=True)
            token = STORE.jwt("KEY123", "issuer-uuid", key.read_text(), at=1_700_000_000)
            head, body, sig = token.split(".")
            raw = STORE.base64.urlsafe_b64decode(sig + "=" * (-len(sig) % 4))
            self.assertEqual(len(raw), 64)

            def integer(b):
                b = b.lstrip(b"\0")
                b = b"\0" + b if b[0] & 0x80 else b
                return bytes([0x02, len(b)]) + b

            seq = integer(raw[:32]) + integer(raw[32:])
            pathlib.Path(tmp, "sig").write_bytes(bytes([0x30, len(seq)]) + seq)
            pathlib.Path(tmp, "msg").write_bytes(f"{head}.{body}".encode())
            ok = subprocess.run(["openssl", "dgst", "-sha256", "-verify", str(pub), "-signature",
                                 str(pathlib.Path(tmp, "sig")), str(pathlib.Path(tmp, "msg"))], capture_output=True)
            self.assertEqual(ok.returncode, 0, ok.stdout + ok.stderr)
            claims = STORE.json.loads(STORE.base64.urlsafe_b64decode(body + "=" * (-len(body) % 4)))
            self.assertEqual(claims, {"iss": "issuer-uuid", "iat": 1_700_000_000, "exp": 1_700_000_900,
                                      "aud": "appstoreconnect-v1"})


if __name__ == "__main__":
    unittest.main()

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
        for app_state in ["PREPARE_FOR_SUBMISSION", "READY_FOR_REVIEW", "DEVELOPER_REJECTED", "REJECTED",
                          "METADATA_REJECTED"]:
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
    def decide(self, versions, builds, submissions=(), min_age=24, number=0, manual=False):
        return STORE.decide(list(versions), list(submissions), STORE.builds_of(builds), NOW, min_age, number, manual)

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

    def test_beta_review_and_testers_play_no_part(self):
        # No regular TestFlight testers: a valid build a day old goes, whatever its beta state.
        for external in ["IN_BETA_TESTING", "WAITING_FOR_BETA_REVIEW", "IN_BETA_REVIEW", "READY_FOR_BETA_SUBMISSION",
                         "BETA_REJECTED", "NOT_APPLICABLE", None]:
            with self.subTest(external):
                plan, _ = self.decide([ON_SALE], builds_response((825, "1.0.1", 30, "VALID", external)))
                self.assertEqual(plan["build"]["number"], 825)

    def test_a_build_waits_a_day_after_its_upload(self):
        builds = builds_response((830, "1.0.1", 23.9, "VALID", "IN_BETA_TESTING"))
        plan, why = self.decide([ON_SALE], builds)
        self.assertIsNone(plan)
        self.assertIn("Nothing to submit", why)
        self.assertEqual(self.decide([ON_SALE], builds, min_age=24 - 0.1)[0]["build"]["number"], 830)

    def test_an_expired_build_is_never_chosen(self):
        builds = builds_response((830, "1.0.1", 30, "VALID", "EXPIRED"))
        builds["data"][0]["attributes"]["expired"] = True
        self.assertIsNone(self.decide([ON_SALE], builds)[0])
        self.assertIsNone(self.decide([ON_SALE], builds, min_age=0, number=830)[0])

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
        for app_state in ["REJECTED", "METADATA_REJECTED", "INVALID_BINARY"]:
            with self.subTest(app_state):
                plan, why = self.decide([ON_SALE, version("v101", "1.0.1", app_state)], builds,
                                        [submission("s1", "UNRESOLVED_ISSUES")])
                self.assertIsNone(plan)
                self.assertIn("1.0.1", why)

    def test_a_manual_run_resubmits_a_rejected_version_with_a_newer_build(self):
        builds = builds_response((830, "1.0.2", 3, "VALID", "IN_BETA_TESTING"))
        rejected = version("v101", "1.0.1", "REJECTED")
        unresolved = submission("s1", "UNRESOLVED_ISSUES")
        plan, _ = self.decide([ON_SALE, rejected], builds, [unresolved], min_age=0, manual=True)
        self.assertIs(plan["reuse"], rejected)
        self.assertIs(plan["draft"], unresolved)
        self.assertTrue(plan["resubmit"])
        self.assertEqual(plan["build"]["version"], "1.0.2")
        # An invalid binary is still for App Store Connect alone.
        plan, _ = self.decide([ON_SALE, version("v101", "1.0.1", "INVALID_BINARY")], builds, min_age=0, manual=True)
        self.assertIsNone(plan)

    def test_a_version_left_on_an_unsent_submission_is_resumed(self):
        # A run added 1.0.1 to a draft submission and stopped before sending it.
        builds = builds_response((825, "1.0.1", 30, "VALID", "IN_BETA_TESTING"))
        waiting = version("v101", "1.0.1", "READY_FOR_REVIEW")
        draft = submission("s1", "READY_FOR_REVIEW")
        plan, _ = self.decide([ON_SALE, waiting], builds, [draft])
        self.assertIs(plan["reuse"], waiting)
        self.assertIs(plan["draft"], draft)
        self.assertFalse(plan["resubmit"])

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
        builds = builds_response((830, "1.0.1", 2, "VALID", "IN_BETA_REVIEW"),
                                 (828, "1.0.1", 1, "VALID", "IN_BETA_REVIEW"),
                                 (827, "1.0.1", 1, "PROCESSING", None))
        self.assertEqual(self.decide([ON_SALE], builds, min_age=0)[0]["build"]["number"], 830)
        self.assertEqual(self.decide([ON_SALE], builds, min_age=0, number=828)[0]["build"]["number"], 828)
        self.assertIsNone(self.decide([ON_SALE], builds, min_age=0, number=827)[0])
        self.assertIsNone(self.decide([ON_SALE], builds, min_age=0, number=999)[0])

    def test_the_first_release_has_no_update_steps(self):
        plan, _ = self.decide([], builds_response((800, "1.0.0", 30, "VALID", "IN_BETA_TESTING")))
        self.assertFalse(plan["update"])


class FakeApi:
    """Records writes; answers reads from a table of paths (the longest matching prefix). A
    version read with no entry answers READY_FOR_REVIEW, as App Store Connect does once the
    version is on a submission."""

    def __init__(self, reads):
        self.reads, self.writes, self.version_reads = reads, [], 0

    def call(self, method, path, body=None):
        if method == "GET":
            keys = [k for k in self.reads if path.startswith(k)]
            if keys:
                answer = self.reads[max(keys, key=len)]
                if isinstance(answer, Exception):
                    raise answer
                return answer() if callable(answer) else answer
            if path.startswith("/v1/appStoreVersions/") and path.endswith("/build"):
                return {"data": {"type": "builds", "id": "b800", "attributes": {"version": "800"}}}
            if path.startswith("/v1/appStoreVersions/") and path.count("/") == 3:
                self.version_reads += 1
                return {"data": version(path.rsplit("/", 1)[1], "1.0.1", "READY_FOR_REVIEW")}
            raise AssertionError(f"unexpected GET {path}")
        self.writes.append((method, path, body))
        if method == "POST":
            return {"data": {"type": body["data"]["type"], "id": f"new-{body['data']['type']}"}}
        return {}

    def get_all(self, path):
        return self.call("GET", path)


LOCALIZATION = {"type": "appStoreVersionLocalizations", "id": "loc-en",
                "attributes": {"locale": "en-US", "whatsNew": None, "description": "The Divine Office."}}
NEW = "/v1/appStoreVersions/new-appStoreVersions"


def item(iid, vid, item_state="READY_FOR_REVIEW"):
    """A reviewSubmissionItems resource as GET .../items?include=appStoreVersion returns it."""
    return {"type": "reviewSubmissionItems", "id": iid, "attributes": {"state": item_state},
            "relationships": {"appStoreVersion": {"data": {"type": "appStoreVersions", "id": vid}}}}


def writes(api):
    return [(m, p) for m, p, _ in api.writes]


class SubmitTest(unittest.TestCase):
    def setUp(self):
        patcher = mock.patch.object(STORE, "sleep", lambda _s: None)
        patcher.start()
        self.addCleanup(patcher.stop)

    def plan(self, versions, submissions=(), encryption=False, manual=False, number=825, string="1.0.1"):
        builds = builds_response((number, string, 30, "VALID", "IN_BETA_TESTING", encryption))
        plan, why = STORE.decide(versions, list(submissions), STORE.builds_of(builds), NOW, 24, 0, manual)
        self.assertIsNotNone(plan, why)
        return plan

    def test_a_missing_phased_release_that_answers_404_is_created(self):
        api = FakeApi({f"{NEW}/appStoreVersionLocalizations": {"data": [LOCALIZATION]},
                       f"{NEW}/appStoreVersionPhasedRelease": STORE.ApiError("404 NOT_FOUND", 404)})
        STORE.submit(api, APP, self.plan([ON_SALE]), "Feast pages redesigned.")
        self.assertIn(("POST", "/v1/appStoreVersionPhasedReleases"), writes(api))

    def test_another_error_reading_the_phased_release_still_stops(self):
        api = FakeApi({f"{NEW}/appStoreVersionLocalizations": {"data": [LOCALIZATION]},
                       f"{NEW}/appStoreVersionPhasedRelease": STORE.ApiError("403 FORBIDDEN", 403)})
        with self.assertRaisesRegex(STORE.ApiError, "403"):
            STORE.submit(api, APP, self.plan([ON_SALE]), "Feast pages redesigned.")

    def test_a_new_version_is_created_filled_in_and_submitted(self):
        api = FakeApi({f"{NEW}/appStoreVersionLocalizations": {"data": [LOCALIZATION]},
                       f"{NEW}/appStoreVersionPhasedRelease": {"data": None}})
        self.assertEqual(STORE.submit(api, APP, self.plan([ON_SALE]), "Feast pages redesigned."), "new-appStoreVersions")
        self.assertEqual(writes(api), [
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
        self.assertEqual(api.writes[2][2]["data"]["attributes"], {"phasedReleaseState": "INACTIVE"})
        self.assertEqual(api.writes[2][2]["data"]["relationships"]["appStoreVersion"]["data"]["id"], "new-appStoreVersions")
        rel = api.writes[4][2]["data"]["relationships"]
        self.assertEqual(rel["reviewSubmission"]["data"]["id"], "new-reviewSubmissions")
        self.assertEqual(rel["appStoreVersion"]["data"]["id"], "new-appStoreVersions")
        self.assertEqual(api.writes[5][2]["data"]["attributes"], {"submitted": True})
        self.assertEqual(api.version_reads, 1)

    def test_it_waits_for_the_version_to_be_ready_before_sending(self):
        answers = iter(["PREPARE_FOR_SUBMISSION", "PREPARE_FOR_SUBMISSION", "READY_FOR_REVIEW"])
        api = FakeApi({f"{NEW}/appStoreVersionLocalizations": {"data": [LOCALIZATION]},
                       f"{NEW}/appStoreVersionPhasedRelease": {"data": None},
                       NEW: lambda: {"data": version("new-appStoreVersions", "1.0.1", next(answers))}})
        STORE.submit(api, APP, self.plan([ON_SALE]), "Notes.")
        self.assertEqual(writes(api)[-1], ("PATCH", "/v1/reviewSubmissions/new-reviewSubmissions"))

    def test_a_version_that_never_becomes_ready_is_not_sent(self):
        api = FakeApi({f"{NEW}/appStoreVersionLocalizations": {"data": [LOCALIZATION]},
                       f"{NEW}/appStoreVersionPhasedRelease": {"data": None},
                       NEW: {"data": version("new-appStoreVersions", "1.0.1", "PREPARE_FOR_SUBMISSION")}})
        with self.assertRaisesRegex(STORE.ApiError, "not ready for review"):
            STORE.submit(api, APP, self.plan([ON_SALE]), "Notes.")
        self.assertNotIn(("PATCH", "/v1/reviewSubmissions/new-reviewSubmissions"), writes(api))

    def test_a_run_that_stopped_before_sending_is_finished(self):
        # The first run added 1.0.1 to a draft submission, then failed to send it: App Store
        # Connect shows the version and the submission as ready for review.
        waiting = version("v101", "1.0.1", "READY_FOR_REVIEW")
        draft = submission("s1", "READY_FOR_REVIEW")
        done = dict(LOCALIZATION, attributes=dict(LOCALIZATION["attributes"], whatsNew="Notes."))
        api = FakeApi({"/v1/appStoreVersions/v101/appStoreVersionLocalizations": {"data": [done]},
                       "/v1/appStoreVersions/v101/appStoreVersionPhasedRelease": {"data": {
                           "type": "appStoreVersionPhasedReleases", "id": "p1",
                           "attributes": {"phasedReleaseState": "INACTIVE"}}},
                       "/v1/reviewSubmissions/s1/items?include=appStoreVersion": {"data": [item("i1", "v101")]}})
        STORE.submit(api, APP, self.plan([ON_SALE, waiting], [draft]), "Notes.")
        self.assertEqual(writes(api), [
            ("PATCH", "/v1/appStoreVersions/v101/relationships/build"),
            ("PATCH", "/v1/reviewSubmissions/s1"),
        ])
        self.assertEqual(api.writes[0][2], {"data": {"type": "builds", "id": "build-825"}})

    def test_a_draft_holding_something_else_is_left_alone(self):
        prepared = version("v101", "1.0.1", "PREPARE_FOR_SUBMISSION")
        draft = submission("s1", "READY_FOR_REVIEW")
        event = {"type": "reviewSubmissionItems", "id": "i9", "attributes": {"state": "READY_FOR_REVIEW"},
                 "relationships": {"appStoreVersion": {"data": None},
                                   "appEvent": {"data": {"type": "appEvents", "id": "e1"}}}}
        for held in [[event], [item("i8", "v999")]]:
            with self.subTest(held=held[0]["id"]):
                api = FakeApi({"/v1/reviewSubmissions/s1/items?include=appStoreVersion": {"data": held}})
                with contextlib.redirect_stderr(io.StringIO()):
                    self.assertIsNone(STORE.submit(api, APP, self.plan([ON_SALE, prepared], [draft]), "Notes."))
                self.assertEqual(api.writes, [])

    def test_a_rejected_version_is_renumbered_and_resubmitted(self):
        rejected = version("v101", "1.0.1", "REJECTED")
        unresolved = submission("s1", "UNRESOLVED_ISSUES")
        api = FakeApi({"/v1/appStoreVersions/v101/appStoreVersionLocalizations": {"data": [LOCALIZATION]},
                       "/v1/appStoreVersions/v101/appStoreVersionPhasedRelease": {"data": {
                           "type": "appStoreVersionPhasedReleases", "id": "p1",
                           "attributes": {"phasedReleaseState": "INACTIVE"}}},
                       "/v1/reviewSubmissions/s1/items?include=appStoreVersion": {
                           "data": [item("i1", "v101", "REJECTED")]}})
        plan = self.plan([ON_SALE, rejected], [unresolved], manual=True, number=830, string="1.0.2")
        STORE.submit(api, APP, plan, "Notes.")
        self.assertEqual(writes(api), [
            ("PATCH", "/v1/appStoreVersions/v101"),
            ("PATCH", "/v1/appStoreVersions/v101/relationships/build"),
            ("PATCH", "/v1/appStoreVersionLocalizations/loc-en"),
            ("PATCH", "/v1/reviewSubmissionItems/i1"),
            ("PATCH", "/v1/reviewSubmissions/s1"),
        ])
        self.assertEqual(api.writes[0][2]["data"]["attributes"], {"versionString": "1.0.2"})
        self.assertEqual(api.writes[3][2]["data"]["attributes"], {"resolved": True})
        self.assertEqual(api.version_reads, 0)

    def test_a_prepared_version_is_renumbered_and_set_to_release_on_approval(self):
        prepared = version("v101", "1.0.0", "DEVELOPER_REJECTED", release="MANUAL")
        api = FakeApi({})
        STORE.submit(api, APP, self.plan([prepared]), "Notes.")
        self.assertEqual(api.writes[0][:2], ("PATCH", "/v1/appStoreVersions/v101"))
        self.assertEqual(api.writes[0][2]["data"]["attributes"], {"versionString": "1.0.1", "releaseType": "AFTER_APPROVAL"})

    def test_no_localizations_is_warned_about(self):
        api = FakeApi({f"{NEW}/appStoreVersionLocalizations": {"data": []},
                       f"{NEW}/appStoreVersionPhasedRelease": {"data": None}})
        err = io.StringIO()
        with contextlib.redirect_stderr(err):
            STORE.submit(api, APP, self.plan([ON_SALE]), "Notes.")
        self.assertIn("::warning::Version 1.0.1 has no localizations", err.getvalue())

    def test_export_compliance_is_answered_only_when_missing(self):
        api = FakeApi({f"{NEW}/appStoreVersionLocalizations": {"data": [LOCALIZATION]},
                       f"{NEW}/appStoreVersionPhasedRelease": {"data": None}})
        STORE.submit(api, APP, self.plan([ON_SALE], encryption=None), "Notes.")
        patch = [b for m, p, b in api.writes if p == "/v1/builds/build-825"]
        self.assertEqual(patch[0]["data"]["attributes"], {"usesNonExemptEncryption": False})


class RetryTest(unittest.TestCase):
    def call(self, method, failures):
        """Api.call against urlopen failing with each of `failures` in turn, then answering."""
        sequence = list(failures)
        calls = []

        class Response(io.BytesIO):
            def __enter__(self):
                return self

            def __exit__(self, *exc):
                return False

        def urlopen(req, timeout):
            calls.append(timeout)
            if sequence:
                failure = sequence.pop(0)
                if isinstance(failure, int):
                    raise STORE.urllib.error.HTTPError(req.full_url, failure, "x", {}, io.BytesIO(b"{}"))
                raise failure
            return Response(b'{"data": []}')

        api = STORE.Api("K", "I", "P")
        api.token, api.made = "t", STORE.time.time()
        with mock.patch.object(STORE.urllib.request, "urlopen", urlopen), \
                mock.patch.object(STORE, "sleep", lambda _s: None), \
                contextlib.redirect_stderr(io.StringIO()):
            try:
                return api.call(method, "/v1/apps", None if method == "GET" else {"data": {}}), calls
            except STORE.ApiError as e:
                return e, calls

    def test_reads_are_retried_with_a_timeout(self):
        result, calls = self.call("GET", [503, STORE.urllib.error.URLError("reset"), TimeoutError("slow")])
        self.assertEqual(result, {"data": []})
        self.assertEqual(calls, [STORE.TIMEOUT] * 4)

    def test_rate_limits_are_retried_for_writes_too(self):
        self.assertEqual(self.call("PATCH", [429])[0], {"data": []})

    def test_writes_are_not_repeated_after_a_server_error_or_no_answer(self):
        for failure in [500, STORE.urllib.error.URLError("reset")]:
            result, calls = self.call("POST", [failure])
            self.assertIsInstance(result, STORE.ApiError)
            self.assertEqual(len(calls), 1)

    def test_a_refusal_is_not_retried_and_retries_run_out(self):
        result, calls = self.call("GET", [409])
        self.assertIsInstance(result, STORE.ApiError)
        self.assertEqual(len(calls), 1)
        result, calls = self.call("GET", [503] * (STORE.RETRIES + 1))
        self.assertIsInstance(result, STORE.ApiError)
        self.assertEqual(len(calls), STORE.RETRIES + 1)


class MainTest(unittest.TestCase):
    def run_main(self, argv, reads):
        # commit_of finds nothing: the default notes, never HEAD's notes, are used.
        api = FakeApi(reads)
        out, err = io.StringIO(), io.StringIO()
        with mock.patch.object(STORE, "Api", lambda *a, **k: api), \
                mock.patch.object(STORE, "now", lambda: NOW), \
                mock.patch.object(STORE, "commit_of", lambda n: None), \
                mock.patch.object(STORE.release_notes, "release_text", lambda *a, **k: "Notes from HEAD."), \
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
        self.assertNotIn("Notes from HEAD.", err)

    def test_tag_output_emitted_only_when_untagged(self):
        reads = dict(self.READS)
        reads[f"/v1/apps/{APP}/appStoreVersions"] = {"data": [ON_SALE]}
        reads["/v1/appStoreVersions/v100/build"] = {"data": {"type": "builds", "id": "b1", "attributes": {"version": "800"}}}
        reads["/v1/reviewSubmissions"] = {"data": []}
        reads["/v1/builds"] = builds_response((800, "1.0.0", 30, "VALID", "IN_BETA_TESTING"))
        for already_tagged, expected in ((False, [{"version": "1.0.0", "build": "800"}]), (True, [])):
            emitted = []
            with mock.patch.object(STORE.release_notes, "has_tag", lambda _p, _v, _b: already_tagged), \
                    mock.patch.object(STORE, "output", lambda **kw: emitted.append(kw)):
                self.run_main(["auto"], reads)
            self.assertEqual(emitted, expected)


class ReleaseNotesTest(unittest.TestCase):
    def test_live_build_reads_highest_released_version_and_handles_errors(self):
        older = version("v090", "0.9.0", "REPLACED_WITH_NEW_VERSION")
        api = FakeApi({"/v1/appStoreVersions/v100/build": {"data": {"attributes": {"version": "800"}}}})
        self.assertEqual(STORE.live_build(api, [older, ON_SALE]), ("1.0.0", 800))
        self.assertIsNone(STORE.live_build(api, [version("v101", "1.0.1", "WAITING_FOR_REVIEW")]))

        class BrokenApi:
            def call(self, _m, _p):
                raise STORE.ApiError("503", 503)

        with contextlib.redirect_stderr(io.StringIO()):
            self.assertIsNone(STORE.live_build(BrokenApi(), [ON_SALE]))

    def test_notes_between_uses_live_build_then_tag_fallback(self):
        self.assertEqual(STORE.notes_between(800, None), STORE.DEFAULT_NOTES)
        with mock.patch.object(STORE.release_notes, "commit_of", lambda n: "c800" if n == 800 else None), \
                mock.patch.object(STORE.release_notes, "latest_tag", lambda _p, _c: "ctag"), \
                mock.patch.object(STORE.release_notes, "release_text", lambda p, b, h: f"{p}:{b}..{h}"):
            self.assertEqual(STORE.notes_between(800, "c900"), "ios:c800..c900")
            self.assertEqual(STORE.notes_between(999, "c900"), "ios:ctag..c900")
            self.assertEqual(STORE.notes_between(None, "c900"), "ios:ctag..c900")


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

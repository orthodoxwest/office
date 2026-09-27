#!/usr/bin/env python3
"""Static PWA/CSS contracts complement the browser behavior and visual suites."""
from pathlib import Path
import re
import unittest

STATIC = Path(__file__).resolve().parent.parent / "apps/office-web/static"


class StaticContracts(unittest.TestCase):
    def required(self, body, tokens):
        for token in tokens:
            with self.subTest(token=token):
                self.assertIn(token, body)

    def forbidden(self, body, tokens):
        for token in tokens:
            with self.subTest(token=token):
                self.assertNotIn(token, body)

    def test_navigation(self):
        body = (STATIC / 'sw.js').read_text()
        self.required(body, [
            '"/reminders"',
            '"/calendar/" + years[y]',
            '"/?date=" + slug',
            '"/" + HOURS[h] + "/" + slug',
            'todayShellURLs',
            'datedEquivalent',
            'isSWRPage',
            'staleWhileRevalidate',
            'redirectToDated',
            'normalizePathname',
            'canonicalCacheKey',
            'PAGE_NETWORK_TIMEOUT_MS = 2500',
            'fetchWithTimeout',
            'plain.searchParams.delete("theme")',
            'class=\\"offline-page\\"',
            '/static/style.css',
            'Open saved home page',
            'cacheFirst',
            'ASSET_Q',
            'assetURL',
            'cache: "reload"',
            'eb-garamond-regular.woff2',
            'return self.skipWaiting();',
            'precacheCore(cache)',
            '"#d-" + today',
            'return "/" + hour + "/" + qDate',
        ])

    def test_offline_fallback(self):
        body = (STATIC / 'sw.js').read_text()
        self.forbidden(body, [
            'class=\\"site-banner\\"',
            'data-dismiss-banner',
            'not yet been fully checked against the printed books',
            'isKnownOffline',
            'navigator.onLine === false',
        ])

    def test_app_controls(self):
        body = (STATIC / 'app.js').read_text()
        self.required(body, [
            'offline-indicator',
            '/sw.js?online-check=',
            'cache: "no-store"',
            'window.addEventListener("online", checkOnline)',
            'window.addEventListener("offline", function ()',
            'recoveryTimer',
            'failStreak',
            'FAIL_STREAK_TO_SHOW',
            'reg.update',
            'updatePrayNow',
            'data-hour',
            'syncDatedNavigation',
            'pageDateSlug',
            'documentDateSlug',
            'ensureTodayControl',
            'data-nav',
            'data-nav="home"',
            '/?date=" + today',
            'not-today-notice',
            'Go to today',
            'brandIsCurrent',
            'aria-current',
            '.date-jump-form',
            'form.querySelector("a.today-link")',
            'lastSyncedDay',
            'visibilitychange',
            'pageshow',
            'e.persisted',
            'lastSyncedDay = localDateSlug(new Date())',
        ])
        self.required(body, [
            'office-theme',
            'data-theme-choice',
        ])
        self.required(body, [
            'office-text-size',
            'data-text-size-choice',
            'data-text-size',
        ])
        self.forbidden(body, [
            'siteBannerDismissed',
            'banner-dismiss',
            'bannerDismissed',
        ])

    def test_routing(self):
        body = (STATIC / "sw.js").read_text()
        self.required(body, [
            'url.searchParams.has("preview")',
            'event.respondWith(previewNetworkOnly(req))',
            'fetch(req, { cache: "no-store" })',
            'function previewOfflineResponse()',
            'if (path === "/")',
            'return "/?date=" + today',
            'if (qDate && DATE_RE.test(qDate))',
            'return "/" + hour + "/" + qDate',
            'return "/" + hour + "/" + today',
            'return "/calendar/" + new Date().getFullYear() + formQuery + "#d-" + today',
            'function networkFetch(req)',
            'cache: "reload"',
            'var revalidate = networkFetch(req)',
            'fetchWithTimeout(req, PAGE_NETWORK_TIMEOUT_MS)',
            'url.pathname.indexOf("/static/") === 0',
            'event.respondWith(cacheFirst(req))',
            'if (url.pathname === "/sw.js")',
            'if (url.pathname === "/office.ics")',
            'no-store(?:\\s*,|$)',
            'function putIfOk',
            'event.respondWith(staleWhileRevalidate(req, url))',
            'event.respondWith(redirectToDated(url, dated))',
        ])
        self.assertLess(body.index('if (url.searchParams.has("preview"))'), body.index('// Static assets: cache-first'))
        preview = body.split('function previewNetworkOnly', 1)[1].split('\n}\n', 1)[0]
        self.assertNotIn('caches.', preview)
        install, activate = body.split('self.addEventListener("install"', 1)[1].split('self.addEventListener("activate"', 1)
        self.assertNotIn('precacheUpcoming', install)
        self.assertIn('precacheUpcoming()', activate)
        self.assertNotIn('return precacheUpcoming()', activate)

    def test_offline_probe_and_explicit_preferences(self):
        body = (STATIC / "app.js").read_text()
        self.assertNotIn('navigator.onLine === false', body)
        offline = body.split('window.addEventListener("offline"', 1)[1].split('});', 1)[0]
        self.assertIn('checkOnline()', offline)
        self.assertNotIn('setOfflineIndicator(true)', offline)
        self.assertEqual(body.count('localStorage.setItem(TEXT_SIZE_KEY'), 1)
        self.assertIn('paintTextSizeChoice(effectiveTextSizeChoice())', body)

    def test_seasonal_ornaments_keep_functional_colors(self):
        body = (STATIC / "style.css").read_text()
        self.required(body, [
            '--ornament: var(--gold);',
            '--ornament-line: var(--gold-line);',
            'body.season-passiontide {',
            'body.season-eastertide {',
            '.cross {\n  color: var(--rubric);',
            'border-top: 3px double var(--ornament-line);',
            'linear-gradient(145deg, var(--ornament-hi) 0%, var(--ornament-lo) 100%)',
        ])
        self.required(body, [
            'outline: 2px solid var(--gold);',
            'color: var(--gold-line);\n  font-size: 0.85em',
            'color-mix(in srgb, var(--gold) 6%, transparent)',
        ])
        for season in ('season-passiontide', 'season-eastertide'):
            self.assertEqual(body.count('body.' + season + ' {'), 3)
        plain = re.sub(r'/\*.*?\*/', '', body, flags=re.S)
        for declaration in re.findall(r'body\.season-[^{]+\{([^}]+)', plain):
            self.required(declaration, ['--ornament:', '--ornament-line:', '--ornament-hi:', '--ornament-lo:'])


if __name__ == '__main__':
    unittest.main()

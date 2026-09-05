package usage

import "strings"

// botTokens are substrings that appear in the user agent of crawlers and
// scripted clients that identify themselves. Matching is on a lowercased
// agent, so every entry here is lowercase.
var botTokens = []string{
	// Search, ads, and social preview fetchers.
	"googlebot", "google-extended", "google-inspectiontool", "adsbot",
	"bingbot", "bingpreview", "msnbot", "slurp", "duckduckbot", "baiduspider",
	"yandex", "sogou", "exabot", "seznambot", "qwantify", "petalbot",
	"applebot", "facebookexternalhit", "facebot", "twitterbot", "linkedinbot",
	"slackbot", "discordbot", "telegrambot", "whatsapp", "pinterest",
	// Archives and SEO/marketing crawlers.
	"ia_archiver", "archive.org_bot", "ahrefs", "semrush", "mj12bot", "dotbot",
	"dataforseo", "blexbot", "serpstat", "screaming frog", "sitebulb",
	// AI and dataset crawlers.
	"gptbot", "chatgpt-user", "oai-searchbot", "claudebot", "claude-web",
	"anthropic-ai", "ccbot", "perplexitybot", "bytespider", "amazonbot",
	"meta-externalagent", "diffbot", "cohere-ai", "timpibot", "youbot",
	// Automation, libraries, and monitoring — never a praying human.
	"headlesschrome", "phantomjs", "puppeteer", "playwright", "selenium",
	"scrapy", "python-requests", "python-urllib", "aiohttp", "httpx",
	"go-http-client", "okhttp", "libwww-perl", "java/", "apache-httpclient",
	"curl/", "wget/", "lighthouse", "pagespeed", "uptimerobot", "pingdom",
	"newrelicpinger", "statuscake", "site24x7", "prerender",
	// Generic self-descriptions.
	"crawler", "crawling", "spider", "scraper", "feedfetcher", "monitoring",
}

// IsBot reports whether a user agent belongs to a crawler or scripted client
// rather than a person reading the Office. Beyond the explicit tokens above it
// catches unknown crawlers by the near-universal "<name>bot/<version>" form.
// The bare word "bot" is deliberately NOT enough: phone agents embed it
// ("Cubot Note 20", "CUBOT_X30"), and losing those readers to a tidier
// heuristic would corrupt the count in the opposite direction. An empty agent
// counts as a bot — every real browser sends one, so omitting it is itself a
// scripted-client signal.
func IsBot(agent string) bool {
	agent = strings.ToLower(strings.TrimSpace(agent))
	if agent == "" {
		return true
	}
	if strings.Contains(agent, "bot/") {
		return true
	}
	for _, token := range botTokens {
		if strings.Contains(agent, token) {
			return true
		}
	}
	return false
}

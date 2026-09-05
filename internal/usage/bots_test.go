package usage

import "testing"

func TestIsBot(t *testing.T) {
	bots := []string{
		"",
		"   ",
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
		"Mozilla/5.0 (compatible; AhrefsBot/7.0; +http://ahrefs.com/robot/)",
		"Mozilla/5.0 (compatible; SemrushBot/7~bl; +http://www.semrush.com/bot.html)",
		"GPTBot/1.1 (+https://openai.com/gptbot)",
		"Mozilla/5.0 (compatible; ClaudeBot/1.0; +claudebot@anthropic.com)",
		"Mozilla/5.0 (compatible; PerplexityBot/1.0)",
		"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/141.0.0.0 Safari/537.36",
		"python-requests/2.32.3",
		"curl/8.7.1",
		"Wget/1.21.4",
		"Go-http-client/2.0",
		"Scrapy/2.11 (+https://scrapy.org)",
		"facebookexternalhit/1.1",
		"Mozilla/5.0 (compatible; YandexBot/3.0)",
		// An unknown crawler still following the "<name>bot/<version>" form.
		"Mozilla/5.0 (compatible; NewfangledBot/0.3; +https://example.test)",
	}
	for _, agent := range bots {
		if !IsBot(agent) {
			t.Errorf("IsBot(%q) = false, want true", agent)
		}
	}

	// People. The phone entries matter most: several Android makers put "bot"
	// inside a model name, and dropping those readers would skew the count the
	// other way. "Abbot"/"Talbot" stand in for the same hazard in any UA token.
	humans := []string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0",
		"Mozilla/5.0 (Linux; Android 13; Cubot Note 20) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Mobile Safari/537.36",
		"Mozilla/5.0 (Linux; Android 10; CUBOT_X30) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Mobile Safari/537.36",
		"Mozilla/5.0 (Linux; Android 14; Abbot One) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Mobile Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Safari/605.1.15",
	}
	for _, agent := range humans {
		if IsBot(agent) {
			t.Errorf("IsBot(%q) = true, want false", agent)
		}
	}
}

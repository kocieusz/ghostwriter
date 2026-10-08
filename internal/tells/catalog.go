// Package tells finds the patterns that make prose read as machine-written.
//
// Every rule carries a strength: one sighting of a hard tell fails a draft
// outright, strong tells justify an edit on their own, and weak tells only
// count when other tells keep them company.
// Counts are judged against the writer's own baseline (see Baseline), so a
// writer who really does use em dashes or "in conclusion" keeps them.
package tells

import "regexp"

// Strength orders how much one sighting of a pattern says about AI drafting.
type Strength int

const (
	Weak Strength = iota + 1
	Medium
	Strong
	Hard
)

func (s Strength) String() string {
	return [...]string{"", "weak", "medium", "strong", "hard"}[s]
}

// Rule is one family of tells.
type Rule struct {
	ID       string
	Name     string
	Strength Strength
	Fix      string // what to do instead, phrased as an instruction to the drafter
	// Raw rules run on the unnormalised text (curly quotes, citation markup);
	// all others run on text with code, URLs and quoted speech masked out.
	Raw   bool
	re    *regexp.Regexp
	check func(text string) [][2]int // custom detector returning byte spans
}

func rx(pattern string) *regexp.Regexp { return regexp.MustCompile(pattern) }

// Rules is the full catalogue, strongest families first.
var Rules = []*Rule{
	// --- hard: leftovers no human draft contains --------------------------
	{
		ID: "chat_artifact", Name: "chatbot citation or markup artifact", Strength: Hard, Raw: true,
		Fix: "Delete the tool markup (oaicite, turn0search, [cite: n], utm_source=chatgpt.com …).",
		re:  rx(`contentReference\[oaicite:\d+\]|oai_citation|\bturn\d+(?:search|image|news|file)\d+|\[cite:\s*\d[^\]]*\]|\[attached_file:\d+\]|\[web:\d+\]|grok_card|grok_render_citation|utm_source=(?:chatgpt\.com|openai|copilot\.com)|:::writing\{|\[span_\d+\]\((?:start|end)_span\)|ppl-ai-file-upload`),
	},
	{
		ID: "placeholder", Name: "unfilled template placeholder", Strength: Hard,
		Fix: "Fill in or remove the placeholder; never leave [Your Name] or 2025-xx-xx in a draft.",
		re:  rx(`(?i)\[(?:your|insert|recipient(?:'s)?|company|client|author|entertainer(?:'s)?|name of|date)\b[^\]\n]{0,30}\]|\b(?:19|20)\d\d-xx-xx\b|\((?:add|insert) [^)\n]{1,40} here\)`),
	},
	{
		ID: "ai_disclosure", Name: "model self-reference or knowledge-cutoff note", Strength: Hard,
		Fix: "Remove any mention of being an AI, training data, or a knowledge cutoff.",
		re:  rx(`(?i)\bas an ai\b|\bas a (?:large )?language model\b|\bas of my (?:last|latest) (?:knowledge |training )?(?:update|cutoff)\b|\bup to my (?:last )?training\b|\bmy (?:knowledge|training) cutoff\b|\bi (?:don't|do not|cannot|can't) (?:browse|access the internet)\b`),
	},

	// --- strong: staging instead of stating ----------------------------
	{
		ID: "negative_parallelism", Name: `"not X but Y" contrast`, Strength: Strong,
		Fix: "State the point directly. Keep a contrast only if the reader actually believes the negated half.",
		re: rx(`(?i)\bnot (?:just|only|merely|simply)\b[^.!?\n]{1,80}?\bbut\b` +
			`|\b(?:it|this|that)(?:'s| is| was) not (?:about )?[^.!?\n]{1,60}?[,;:—–-]+\s*(?:it|this|that)(?:'s| is| was)\b` +
			`|\b(?:it|this|that) (?:isn't|wasn't) (?:just |only |about )?[^.!?\n]{1,60}?[,;:—–-]+\s*(?:it|this|that)(?:'s| is| was)\b` +
			`|\b(?:this|that|it) (?:does not|doesn't) mean\b[^.!?\n]{0,120}[.!?]\s+(?:it|this|that) means\b` +
			`|\bno [^,.!?\n]{1,25}, no [^,.!?\n]{1,25}(?:, no [^,.!?\n]{1,25})?[,.]? just\b`),
	},
	{
		ID: "dramatic_closer", Name: "one-line closer or dramatic fragments", Strength: Strong,
		Fix: "Cut lines that restate or announce the point (\"That's the real win.\"); merge fragment rows into one claim.",
		re: rx(`(?i)\b(?:that'?s|this is|here'?s) the (?:real|whole|actual) (?:win|story|lesson|point|magic|unlock|trick|secret)\b` +
			`|\blet that sink in\b|\bread that again\b|\bthat distinction matters\b|\bthe message (?:was|is) clear\b` +
			`|\bthis (?:shows|highlights|underscores|demonstrates|illustrates) the importance of\b|\bit was a lesson in\b` +
			`|\band (?:that|this) changes everything\b|\bfull stop\.|\bend of story\.` +
			`|(?:\bNo [\p{L}'][^.!?\n]{0,30}\.\s+){2,}` +
			`|(?:^|\s)[a-z]+\. [a-z]+\. [a-z]+\.(?:\s|$)`),
	},
	{
		ID: "deep_saying", Name: "saying dressed as insight", Strength: Strong,
		Fix: "Replace the aphorism (\"at its core\", \"X is the language of Y\") with the specific claim.",
		re: rx(`(?i)\bthe real (?:question|issue|problem|story) (?:is|here)\b|\bat (?:its|the) (?:very )?core\b|\bwhat (?:really|truly) matters\b` +
			`|\bthe heart of the matter\b|\bthe deeper (?:issue|question|truth)\b|\bis (?:the|a) (?:language|currency|architecture|grammar) of\b` +
			`|\bbecomes a trap\b|\bin reality,|\bfundamentally,`),
	},
	{
		ID: "staged_opener", Name: "staged run-up before the point", Strength: Strong,
		Fix: "Delete the announcement (\"Let's dive in\", \"Here's the thing\") and start with the point.",
		re: rx(`(?i)\blet'?s (?:dive|delve|dig) (?:in|into|deeper)\b|\blet'?s (?:explore|unpack|break (?:this|it|that) down)\b` +
			`|\bhere'?s (?:what you need to know|the thing|the kicker|the catch|why (?:this|it) matters)\b|\bwithout further ado\b` +
			`|\bthe thing is,|\blet'?s be honest\b|\breal talk\b|(?:^|[.!?]\s+)honestly\?|\bbuckle up\b` +
			`|\bin this (?:article|post|guide|blog post|piece),? (?:we|i)(?:'ll| will)\b`),
	},
	{
		ID: "strawman", Name: "arguing with no one", Strength: Strong,
		Fix: "Drop the defence against an objection nobody raised; state the claim it was hiding.",
		re: rx(`(?i)\bthis (?:isn't|is not) (?:mainly |just |only |really )?about\b|\bi'?m not saying\b|\bto be clear,|\bdon'?t get me wrong\b` +
			`|\bthis is not to say\b|\bone might be tempted\b|\ba tempting (?:approach|option|answer)\b|\bsome (?:might|may|would) (?:say|argue)\b`),
	},
	{
		ID: "chatbot_residue", Name: "assistant wrapper (greeting, praise, offer)", Strength: Strong,
		Fix: "Remove the assistant wrapper and keep only the content.",
		re: rx(`(?i)\bi hope this helps\b|\bgreat question\b|\byou'?re absolutely right\b|\b(?:would you like|do you want) me to\b` +
			`|\bshould i continue\?|\bhappy to help\b|\bi'?d be happy to\b|\bhere(?:'s| is) (?:a|an|the|your) (?:revised|updated|polished|rewritten|refined|improved) \b` +
			`|\blet me know if you(?:'d| would) like (?:me|any)\b|(?m)^(?:certainly|absolutely|of course|sure thing)!`),
	},
	{
		ID: "source_hedge", Name: "source-availability disclaimer", Strength: Strong,
		Fix: "Say plainly what the source does not show, or cut the sentence. Do not guess to fill the gap.",
		re: rx(`(?i)\bwhile (?:specific )?(?:details|information) (?:is|are) (?:limited|scarce)\b|\bnot (?:widely|extensively|publicly|readily) (?:documented|available|disclosed|reported)\b` +
			`|\bbased on (?:the )?available information\b|\bmaintains a low profile\b|\bkeeps (?:his|her|their) personal (?:life|details) private\b|\bin the (?:provided|available) sources\b`),
	},

	// --- medium: inflation and borrowed authority ----------------------
	{
		ID: "inflated_significance", Name: "inflated significance or legacy", Strength: Medium,
		Fix: "Keep the fact, drop the claim that it marks a turning point or legacy. End on the last concrete fact.",
		re: rx(`(?i)\bstands as a testament\b|\ba testament to\b|\b(?:a )?(?:pivotal|crucial|defining|watershed) moment\b|\bplays? an? (?:key|crucial|vital|pivotal|significant|important) role\b` +
			`|\bunderscores? (?:the|its) (?:importance|significance)\b|\breflects a broader\b|\b(?:enduring|lasting) (?:legacy|impact)\b|\bsetting the stage for\b` +
			`|\b(?:evolving|changing|shifting) landscape\b|\bindelible mark\b|\bdespite (?:these|its|their|the) challenges\b|\bcontinues to thrive\b` +
			`|\bthe future looks bright\b|\bexciting times (?:lie )?ahead\b|\ba step in the right direction\b|\bin today'?s (?:fast-paced|digital|ever-changing|rapidly evolving|modern)\b` +
			`|\bever-(?:evolving|changing)\b|\bnavigat(?:e|ing) the complexities\b|\bgame[- ]changer\b|\bparadigm shift\b|\bawards and recognition\b|\bfuture (?:outlook|prospects)\b`),
	},
	{
		ID: "ing_rider", Name: "shallow -ing analysis tacked onto a fact", Strength: Medium,
		Fix: "Cut the trailing \", highlighting/ensuring/reflecting …\" clause unless the source says it.",
		re:  rx(`(?i),\s+(?:thereby\s+)?(?:highlighting|underscoring|emphasi[sz]ing|ensuring|reflecting|symboli[sz]ing|contributing to|cultivating|fostering|encompassing|showcasing|solidifying|cementing|paving the way)\b`),
	},
	{
		ID: "sales_language", Name: "advertising tone", Strength: Medium,
		Fix: "Say what the thing is instead of selling it.",
		re: rx(`(?i)\bnestled\b|\bin the heart of\b|\bbreathtaking\b|\bmust-(?:visit|see|have)\b|\bstunning\b|\brich (?:cultural|history|heritage|tapestry|tradition)\b` +
			`|\brenowned\b|\bgroundbreaking\b|\bdiverse (?:array|range)\b|\bcommitment to (?:excellence|quality|innovation)\b|\bseamless(?:ly)?\b` +
			`|\bunlock (?:the|your|new)\b|\belevate (?:your|the)\b|\bempower(?:s|ing)? (?:you|users|teams)\b|\bcutting[- ]edge\b|\bworld[- ]class\b`),
	},
	{
		ID: "borrowed_authority", Name: "vague attribution or prestige list", Strength: Medium,
		Fix: "Name who said what, or cut the claim. Unnamed experts and lists of outlets prove nothing.",
		re: rx(`(?i)\bexperts (?:argue|believe|say|agree|note|suggest)\b|\bobservers (?:have )?(?:noted|cited|said)\b|\bindustry reports\b|\bsome critics\b` +
			`|\bactive social media presence\b|\bindependent coverage\b|\bwidely (?:regarded|recognized|recognised|considered|acclaimed)\b|\bstudies (?:show|suggest|have shown)\b`),
	},
	{
		ID: "ai_vocabulary", Name: "overused AI vocabulary", Strength: Medium,
		Fix: "Swap the stock AI word for the plain one the writer would use.",
		re: rx(`(?i)\b(?:delv(?:e|es|ed|ing)|tapestry|testament|pivotal|intricate|intricacies|interplay|meticulous(?:ly)?|bolster(?:s|ed|ing)?` +
			`|garner(?:s|ed|ing)?|underscor(?:e|es|ed|ing)|showcas(?:e|es|ed|ing)|vibrant|enduring|crucial|enhanc(?:e|es|ed|ing)|fostering` +
			`|realm|multifaceted|holistic|embark(?:s|ed|ing)?|streamlin(?:e|es|ed|ing)|synerg(?:y|ies)|transformative|unwavering|invaluable` +
			`|commendable|leverag(?:e|es|ed|ing)|align(?:s|ed)? with|landscape|robust|nuanced|resonat(?:e|es|ed|ing)|profound|noteworthy)\b` +
			`|(?m)(?:^|[.!?]\s+)additionally,`),
	},
	{
		ID: "signposting", Name: "filler signposting", Strength: Medium,
		Fix: "Drop the signpost (\"It's worth noting\", \"In summary\") and just say the thing.",
		re:  rx(`(?i)\bit(?:'s| is) (?:important|worth|crucial|essential) (?:to note|noting|mentioning|to remember)\b|\bin (?:summary|conclusion|essence)\b|\bto summari[sz]e\b|\bultimately,|\boverall,|\bat the end of the day\b`),
	},
	{
		ID: "meta_writing", Name: "writing about the document instead of the subject", Strength: Medium,
		Fix: "Talk about the subject, not about the text (\"This section explores…\", \"the table below\").",
		re:  rx(`(?i)\bthis (?:article|post|section|guide|document|piece|essay|report) (?:will )?(?:explores?|outlines?|provides?|covers?|delves?|examines?|discusses?|aims?)\b|\bthe (?:table|list|chart) below\b|\bin this (?:article|section|guide|document)\b|\bas (?:mentioned|noted|discussed) (?:above|earlier|previously)\b`),
	},
	{
		ID: "em_dash", Name: "em/en dash as all-purpose connector", Strength: Medium,
		Fix: "Replace dashes with a comma, period, colon or parentheses unless the writer uses them at this rate.",
		re:  rx(`—|\s–\s|\s--\s`),
	},
	{
		ID: "inline_header_list", Name: "list items with bold inline headers", Strength: Medium,
		Fix: "Turn \"- **Label:** text\" lists into prose unless the labels carry information.",
		re:  rx(`(?m)^\s*(?:[-*•]|\d+[.)])\s+\*\*[^*\n]+\*\*\s*:?`),
	},
	{
		ID: "emoji_decoration", Name: "emoji or arrows as decoration", Strength: Medium,
		Fix: "Remove decorative emoji and arrows from headings and list items.",
		re:  rx(`(?m)^\s*(?:[-*#]+\s*)?[\x{1F300}-\x{1FAFF}\x{2600}-\x{27BF}\x{2192}\x{27A1}]`),
	},

	// --- weak: only count in company -------------------------------------
	{
		ID: "triad", Name: "rule-of-three list", Strength: Weak,
		Fix:   "Check each item of the triplet adds something; merge or develop one instead of listing three.",
		check: triads,
	},
	{
		ID: "stacked_qualifiers", Name: "stacked qualifiers", Strength: Weak,
		Fix: "Keep one qualifier at most, and only where there is real doubt.",
		re:  rx(`(?i)\b(?:could|might|may) (?:potentially|possibly|arguably|perhaps)\b|\bit(?:'s| is) (?:also )?possible that\b|\bto some extent\b|\bin some cases,? it may\b|\bgenerally speaking\b`),
	},
	{
		ID: "copula_avoidance", Name: `avoiding "is"/"has"`, Strength: Weak,
		Fix: "Use is, are and has instead of \"serves as\", \"stands as\", \"boasts\".",
		re:  rx(`(?i)\b(?:serves|served|stands|stood|functions|acts|operates) as (?:a|an|the)\b|\bboasts\b|\brefers to (?:a|an|the)\b`),
	},
	{
		ID: "vague_association", Name: "vague connection", Strength: Weak,
		Fix: "Name the actual relationship (founded, works for, caused) instead of \"associated with\".",
		re:  rx(`(?i)\b(?:associated with|in association with|in connection with|closely linked to|is tied to)\b`),
	},
	{
		ID: "bold_decoration", Name: "bold used as decoration", Strength: Weak,
		Fix: "Remove bold that does not mark a real term the reader must find again.",
		re:  rx(`\*\*[^*\n]+\*\*`),
	},
	{
		ID: "curly_quotes", Name: "curly quotes where the writer types straight ones", Strength: Weak, Raw: true,
		Fix: "Use the writer's quote style.",
		re:  rx(`[“”‘’]`),
	},
	{
		ID: "repeated_openers", Name: "same sentence opening three times running", Strength: Weak,
		Fix:   "Merge the sentences or start one with the action instead of the same subject.",
		check: repeatedOpeners,
	},
	{
		ID: "title_case_heading", Name: "Title Case heading", Strength: Weak,
		Fix:   "Use sentence case in headings.",
		check: titleCaseHeadings,
	},
}

// ByID looks a rule up.
func ByID(id string) *Rule {
	for _, r := range Rules {
		if r.ID == id {
			return r
		}
	}
	return nil
}

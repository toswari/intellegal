export type SemanticDiffStatus = "similar" | "changed" | "missing_left" | "missing_right";

export type SemanticDiffRow = {
  key: string;
  title: string;
  leftText: string;
  rightText: string;
  similarity: number;
  status: SemanticDiffStatus;
};

export type DiffLineKind = "same" | "removed" | "added";

export type DiffLine = {
  kind: DiffLineKind;
  text: string;
};

const CATEGORIES: Array<{ key: string; title: string; keywords: string[] }> = [
  { key: "parties", title: "Parties", keywords: ["party", "parties", "customer", "supplier", "vendor", "client"] },
  { key: "scope", title: "Scope", keywords: ["scope", "services", "deliverable", "statement of work", "sow"] },
  { key: "payment", title: "Payment", keywords: ["payment", "invoice", "fee", "price", "billing", "charge"] },
  { key: "term", title: "Term & Termination", keywords: ["term", "termination", "expire", "renewal", "notice"] },
  { key: "obligations", title: "Obligations", keywords: ["shall", "must", "obligation", "responsibility", "comply"] },
  { key: "liability", title: "Liability & Indemnity", keywords: ["liability", "indemn", "damages", "limitation", "cap"] },
  { key: "confidentiality", title: "Confidentiality & Data", keywords: ["confidential", "data", "privacy", "personal data", "security"] },
  { key: "ip", title: "Intellectual Property", keywords: ["intellectual property", "ip", "ownership", "license", "copyright"] },
  { key: "disputes", title: "Disputes & Governing Law", keywords: ["governing law", "jurisdiction", "arbitration", "dispute", "court"] },
  { key: "remedies", title: "Breach & Remedies", keywords: ["breach", "remedy", "default", "injunctive", "specific performance"] }
];

export function buildSemanticDiff(leftText: string, rightText: string): SemanticDiffRow[] {
  const leftParagraphs = toParagraphs(leftText);
  const rightParagraphs = toParagraphs(rightText);

  return CATEGORIES.map((category) => {
    const left = pickCategoryText(leftParagraphs, category.keywords);
    const right = pickCategoryText(rightParagraphs, category.keywords);
    const similarity = jaccardSimilarity(tokenize(left), tokenize(right));

    let status: SemanticDiffStatus = "changed";
    if (!left && !right) {
      status = "similar";
    } else if (!left) {
      status = "missing_left";
    } else if (!right) {
      status = "missing_right";
    } else if (similarity >= 0.65) {
      status = "similar";
    }

    return {
      key: category.key,
      title: category.title,
      leftText: left,
      rightText: right,
      similarity,
      status
    };
  });
}

export function buildLineDiff(
  leftText: string,
  rightText: string,
  maxLines = 700
): { left: DiffLine[]; right: DiffLine[]; truncated: boolean } {
  const rawLeft = normalizeForLines(leftText).split("\n").filter((line) => line.trim().length > 0);
  const rawRight = normalizeForLines(rightText).split("\n").filter((line) => line.trim().length > 0);

  const truncated = rawLeft.length > maxLines || rawRight.length > maxLines;
  const leftLines = rawLeft.slice(0, maxLines);
  const rightLines = rawRight.slice(0, maxLines);

  const n = leftLines.length;
  const m = rightLines.length;
  const dp: number[][] = Array.from({ length: n + 1 }, () => Array.from({ length: m + 1 }, () => 0));

  for (let i = n - 1; i >= 0; i -= 1) {
    for (let j = m - 1; j >= 0; j -= 1) {
      if (leftLines[i] === rightLines[j]) {
        dp[i][j] = dp[i + 1][j + 1] + 1;
      } else {
        dp[i][j] = Math.max(dp[i + 1][j], dp[i][j + 1]);
      }
    }
  }

  const left: DiffLine[] = [];
  const right: DiffLine[] = [];

  let i = 0;
  let j = 0;
  while (i < n && j < m) {
    if (leftLines[i] === rightLines[j]) {
      left.push({ kind: "same", text: leftLines[i] });
      right.push({ kind: "same", text: rightLines[j] });
      i += 1;
      j += 1;
      continue;
    }

    if (dp[i + 1][j] >= dp[i][j + 1]) {
      left.push({ kind: "removed", text: leftLines[i] });
      right.push({ kind: "same", text: "" });
      i += 1;
    } else {
      left.push({ kind: "same", text: "" });
      right.push({ kind: "added", text: rightLines[j] });
      j += 1;
    }
  }

  while (i < n) {
    left.push({ kind: "removed", text: leftLines[i] });
    right.push({ kind: "same", text: "" });
    i += 1;
  }

  while (j < m) {
    left.push({ kind: "same", text: "" });
    right.push({ kind: "added", text: rightLines[j] });
    j += 1;
  }

  return { left, right, truncated };
}

function pickCategoryText(paragraphs: string[], keywords: string[]): string {
  const weighted = paragraphs
    .map((paragraph) => ({
      paragraph,
      score: keywordScore(paragraph.toLowerCase(), keywords)
    }))
    .filter((item) => item.score > 0)
    .sort((a, b) => b.score - a.score)
    .slice(0, 4)
    .map((item) => item.paragraph);

  return weighted.join("\n\n").trim();
}

function keywordScore(text: string, keywords: string[]): number {
  let score = 0;
  for (const keyword of keywords) {
    if (text.includes(keyword)) {
      score += keyword.includes(" ") ? 2 : 1;
    }
  }
  return score;
}

function normalizeForLines(text: string): string {
  return text.replace(/\r\n/g, "\n").replace(/\t/g, " ").replace(/[ ]{2,}/g, " ").trim();
}

function toParagraphs(text: string): string[] {
  const normalized = text.replace(/\r\n/g, "\n").replace(/\u00a0/g, " ");
  return normalized
    .split(/\n\s*\n+/)
    .map((paragraph) => paragraph.trim())
    .filter((paragraph) => paragraph.length >= 20)
    .slice(0, 400);
}

function tokenize(text: string): Set<string> {
  const tokens = text.toLowerCase().match(/[a-z0-9]+/g) ?? [];
  return new Set(tokens.filter((token) => token.length > 2));
}

function jaccardSimilarity(left: Set<string>, right: Set<string>): number {
  if (left.size === 0 && right.size === 0) {
    return 1;
  }
  if (left.size === 0 || right.size === 0) {
    return 0;
  }

  let overlap = 0;
  for (const token of left) {
    if (right.has(token)) {
      overlap += 1;
    }
  }
  const union = left.size + right.size - overlap;
  return union === 0 ? 0 : overlap / union;
}

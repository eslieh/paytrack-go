// Request-sample generation and a tiny syntax highlighter for the docs.

export const LANGS = [
  { id: "curl", label: "cURL", hl: "sh" },
  { id: "node", label: "Node.js", hl: "js" },
  { id: "python", label: "Python", hl: "py" },
  { id: "php", label: "PHP", hl: "php" },
];

const TOKEN_ENV = { key: "PAYTRACK_API_KEY", session: "PAYTRACK_SESSION_TOKEN" };

function buildUrl(base, ep, req) {
  let path = ep.path;
  for (const [k, v] of Object.entries(req.path || {})) path = path.replace(`{${k}}`, v);
  const q = new URLSearchParams(Object.entries(req.query || {}).map(([k, v]) => [k, String(v)])).toString();
  return `${base}${path}${q ? `?${q}` : ""}`;
}

const json = (v, indent = 2) => JSON.stringify(v, null, indent);
const indentLines = (s, pad) => s.split("\n").map((l, i) => (i === 0 ? l : pad + l)).join("\n");

function toPython(v, pad = "") {
  return indentLines(json(v, 4).replace(/: true\b/g, ": True").replace(/: false\b/g, ": False").replace(/: null\b/g, ": None"), pad);
}

function toPhp(v, pad = "") {
  const inner = pad + "  ";
  if (Array.isArray(v)) return `[${v.map((x) => toPhp(x, pad)).join(", ")}]`;
  if (v && typeof v === "object") {
    const rows = Object.entries(v).map(([k, x]) => `${inner}${JSON.stringify(k)} => ${toPhp(x, inner)},`);
    return `[\n${rows.join("\n")}\n${pad}]`;
  }
  return JSON.stringify(v);
}

export function sample(lang, base, ep, req = ep.request || {}) {
  const url = buildUrl(base, ep, req);
  const env = TOKEN_ENV[ep.auth];
  const extra = Object.entries(req.headers || {});
  const body = req.body;
  const m = ep.method;

  if (lang === "curl") {
    const lines = [`curl${m === "GET" ? "" : ` -X ${m}`} ${url}`];
    if (env) lines.push(`-H "Authorization: Bearer $${env}"`);
    for (const [k, v] of extra) lines.push(`-H "${k}: ${v}"`);
    if (body) {
      lines.push(`-H "Content-Type: application/json"`);
      lines.push(`-d '${json(body)}'`);
    }
    return lines.map((l, i) => (i === 0 ? l : `  ${indentLines(l, "  ")}`)).join(" \\\n");
  }

  if (lang === "node") {
    const headers = [];
    if (env) headers.push(`Authorization: \`Bearer \${process.env.${env}}\`,`);
    for (const [k, v] of extra) headers.push(`${JSON.stringify(k)}: ${JSON.stringify(v)},`);
    if (body) headers.push(`"Content-Type": "application/json",`);
    const opts = [];
    if (m !== "GET") opts.push(`method: "${m}",`);
    if (headers.length) opts.push(`headers: {\n    ${headers.join("\n    ")}\n  },`);
    if (body) opts.push(`body: JSON.stringify(${indentLines(json(body), "  ")}),`);
    const call = opts.length ? `fetch("${url}", {\n  ${opts.join("\n  ")}\n})` : `fetch("${url}")`;
    const read = m === "DELETE" ? "console.log(res.status); // 204" : "const data = await res.json();\nconsole.log(data);";
    return `const res = await ${call};\n${read}`;
  }

  if (lang === "python") {
    const headers = [];
    if (env) headers.push(`"Authorization": f"Bearer {os.environ['${env}']}"`);
    for (const [k, v] of extra) headers.push(`${JSON.stringify(k)}: ${JSON.stringify(v)}`);
    const args = [`"${url}"`];
    if (headers.length) args.push(`headers={${headers.join(", ")}}`);
    if (body) args.push(`json=${toPython(body, "    ")}`);
    const imports = env ? "import os\nimport requests" : "import requests";
    const read = m === "DELETE" ? "print(res.status_code)  # 204" : "print(res.json())";
    return `${imports}\n\nres = requests.${m.toLowerCase()}(\n    ${args.join(",\n    ")},\n)\nres.raise_for_status()\n${read}`;
  }

  if (lang === "php") {
    const headers = [];
    if (env) headers.push(`"Authorization: Bearer " . getenv("${env}"),`);
    for (const [k, v] of extra) headers.push(`"${k}: ${v}",`);
    if (body) headers.push(`"Content-Type: application/json",`);
    const opts = [`CURLOPT_RETURNTRANSFER => true,`];
    if (m !== "GET") opts.push(`CURLOPT_CUSTOMREQUEST => "${m}",`);
    if (headers.length) opts.push(`CURLOPT_HTTPHEADER => [\n        ${headers.join("\n        ")}\n    ],`);
    if (body) opts.push(`CURLOPT_POSTFIELDS => json_encode(${toPhp(body, "    ")}),`);
    return `<?php\n$ch = curl_init("${url}");\ncurl_setopt_array($ch, [\n    ${opts.join("\n    ")}\n]);\n$response = json_decode(curl_exec($ch), true);\n$status = curl_getinfo($ch, CURLINFO_HTTP_CODE);`;
  }
  return "";
}

// ------------------------------------------------------------------ highlighter

const KEYWORDS = {
  js: "const|let|await|async|function|return|if|else|new|import|from|export|require|throw|try|catch|true|false|null|undefined",
  py: "import|from|def|return|if|else|elif|not|and|or|in|try|except|raise|True|False|None|with|as",
  php: "function|return|if|else|true|false|null|echo|new|exit|try|catch",
  go: "package|import|func|return|if|else|var|const|type|struct|err|nil|true|false|defer|range|for",
  sh: "curl",
  json: "true|false|null",
};

const esc = (s) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");

export function highlight(code, lang = "json") {
  const comment = lang === "py" || lang === "sh" ? String.raw`(?<=^|\s)#.*` : String.raw`\/\/.*`;
  const kw = KEYWORDS[lang] || KEYWORDS.json;
  const re = new RegExp(
    [
      String.raw`("(?:\\.|[^"\\\n])*"|'(?:\\.|[^'\\])*'|\x60(?:\\.|[^\x60\\])*\x60)(\s*:)?`, // 1 string, 2 key colon
      `(${comment})`, // 3
      String.raw`\b(\d+(?:\.\d+)?)\b`, // 4
      `\\b(${kw})\\b`, // 5
      String.raw`(\$[A-Za-z_]\w*|\bprocess\.env\.\w+)`, // 6 variables
    ].join("|"),
    "gm",
  );
  let out = "";
  let last = 0;
  for (const m of code.matchAll(re)) {
    out += esc(code.slice(last, m.index));
    last = m.index + m[0].length;
    if (m[1] !== undefined) {
      const cls = m[2] && (lang === "json" || lang === "js" || lang === "py" || lang === "php") ? "k" : "s";
      out += `<span class="tok-${cls}">${esc(m[1])}</span>${m[2] ? esc(m[2]) : ""}`;
    } else if (m[3] !== undefined) out += `<span class="tok-c">${esc(m[3])}</span>`;
    else if (m[4] !== undefined) out += `<span class="tok-n">${esc(m[4])}</span>`;
    else if (m[5] !== undefined) out += `<span class="tok-w">${esc(m[5])}</span>`;
    else out += `<span class="tok-v">${esc(m[6])}</span>`;
  }
  return out + esc(code.slice(last));
}

#!/usr/bin/env python3
import os
import re
import sys

CALL = re.compile(r'(Text|Button|Label|accessibilityLabel|navigationTitle|title:)\s*\(')
LITERAL = re.compile(r'"([^"\\]*(?:\\.[^"\\]*)*)"')


def ternary_literals(line):
    """Literais depois de um `?` dentro da chamada: os ramos de `Text(cond ? "A" : "B")` e
    o padrão de `Text(nome ?? "B")`. O padrão de cima só via o literal logo após o `(`."""
    found = []
    for m in CALL.finditer(line):
        depth, i = 1, m.end()
        while i < len(line) and depth:
            depth += {"(": 1, ")": -1}.get(line[i], 0)
            i += 1
        args = line[m.end():i - 1]
        q = args.find("?")
        if q != -1 and not args.lstrip().startswith('"'):
            found += LITERAL.findall(args[q:])
    return found


def check_swift_literals(views_dir="ios/LogNiOS/LogNiOS"):
    pattern = re.compile(r'(Text|Button|Label|accessibilityLabel|navigationTitle|title:)\s*\(\s*"([^"]+)"')
    errors = []

    for root, _, files in os.walk(views_dir):
        for f in files:
            if not f.endswith(".swift"):
                continue
            path = os.path.join(root, f)
            with open(path, "r") as file:
                lines = file.readlines()
                for i, line in enumerate(lines):
                    if "PreviewProvider" in "".join(lines[i:]):
                        break

                    match = pattern.search(line)
                    contents = ([match.group(2)] if match else []) + ternary_literals(line)
                    for content in contents:
                        # No letters outside the interpolations: just symbols or interpolated numbers
                        bare = re.sub(r'\\\((?:[^()]|\([^()]*\))*\)', '', content)
                        if not any(c.isalpha() for c in bare):
                            continue
                        # Some exceptions
                        if "SampleDataNotice" in path or "Preview" in line:
                            continue
                        # Interpolated strings with no real textual content to translate.
                        if content == "\\(vm.globalXp) XP" or content == "\\(core.viewModel.globalXp) XP":
                            pass
                        elif content == "AC" or content == "PROBLEM \\(String(letter))" or "RELÓGIO" in content:
                            pass

                        # Just a simple heuristic: if it has words that are not just variable names
                        words = [w for w in re.findall(r'[a-zA-Z]+', content) if w not in ['vm', 'row', 'core', 'viewModel', 'String', 'letter', 'lives', 'maxLives', 'solvedCount', 'node', 'prerequisites', 'count', 'problemsSolved', 'level', 'xpForLevel', 'challengesCompleted', 'balloonsUp', 'rank', 'solved', 'penalty', 'Int']]

                        if not words or content in ["AC", "XP", "LogN"]:
                            continue

                        errors.append(f"{path}:{i+1}: Found hardcoded UI literal: {line.strip()}")
                        break

    if errors:
        print("Hardcoded UI literals found. Please use the i18n catalog (Str.<Group>.<key>).")
        for e in errors:
            print(e)
        sys.exit(1)
    print("No hardcoded UI literals found!")

if __name__ == "__main__":
    check_swift_literals(*sys.argv[1:2])

#!/usr/bin/env python3
import os
import re
import sys

def check_swift_literals():
    pattern = re.compile(r'(Text|Button|Label|accessibilityLabel|navigationTitle|title:)\s*\(\s*"([^"]+)"')
    errors = []
    
    views_dir = "ios/LogNiOS/LogNiOS"
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
                    if match:
                        content = match.group(2)
                        # If content contains no alphabet letters, it's just symbols or interpolated numbers
                        if not any(c.isalpha() for c in content):
                            continue
                        # Some exceptions
                        if "SampleDataNotice" in path or "Preview" in line:
                            continue
                        # If it's something like "\(lives) / \(maxLives)", it's handled differently, but we should probably ignore it if it doesn't have real textual words. Wait, " / " is just a symbol. What about " XP"?
                        if content == "\\(vm.globalXp) XP" or content == "\\(core.viewModel.globalXp) XP":
                            # We can allow XP or we can translate it. Let's ignore it for now.
                            pass
                        elif content == "AC" or content == "PROBLEM \\(String(letter))" or "RELÓGIO" in content:
                            pass # We can ignore or fix them. Wait, RELÓGIO DA QUESTÃO is a literal!
                        
                        # Just a simple heuristic: if it has words that are not just variable names
                        words = [w for w in re.findall(r'[a-zA-Z]+', content) if w not in ['vm', 'row', 'core', 'viewModel', 'String', 'letter', 'lives', 'maxLives', 'solvedCount', 'node', 'prerequisites', 'count', 'problemsSolved', 'level', 'xpForLevel', 'challengesCompleted', 'balloonsUp', 'rank', 'solved', 'penalty', 'Int']]
                        
                        if not words or content in ["AC", "XP", "LogN"]:
                            continue

                        errors.append(f"{path}:{i+1}: Found hardcoded UI literal: {line.strip()}")
                        
    if errors:
        print("Hardcoded UI literals found. Please use the i18n catalog (Str.<Group>.<key>).")
        for e in errors:
            print(e)
        sys.exit(1)
    print("No hardcoded UI literals found!")

if __name__ == "__main__":
    check_swift_literals()

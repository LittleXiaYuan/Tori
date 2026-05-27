#!/usr/bin/env node

/**
 * Check Commit Type Distribution
 *
 * Analyzes recent commits to report type distribution and identify potential misclassifications.
 *
 * Usage:
 *   node scripts/check-commit-type.mjs [options]
 *
 * Options:
 *   -n, --count <number>    Number of commits to analyze (default: 100)
 *   -v, --verbose           Show detailed analysis
 *   -h, --help              Show help
 */

import { execSync } from 'child_process';

const COMMIT_TYPES = {
  feat: { name: 'Feature', color: '\x1b[32m' },
  fix: { name: 'Bug Fix', color: '\x1b[33m' },
  refactor: { name: 'Refactor', color: '\x1b[36m' },
  perf: { name: 'Performance', color: '\x1b[35m' },
  test: { name: 'Test', color: '\x1b[34m' },
  docs: { name: 'Documentation', color: '\x1b[37m' },
  style: { name: 'Style', color: '\x1b[90m' },
  chore: { name: 'Chore', color: '\x1b[90m' },
  other: { name: 'Other/Invalid', color: '\x1b[31m' }
};

const RESET = '\x1b[0m';
const BOLD = '\x1b[1m';
const DIM = '\x1b[2m';

// Patterns that suggest potential misclassification
const REFACTOR_PATTERNS = [
  /\b(extract|consolidate|rename|reorganize|restructure|simplify|cleanup|clean up)\b/i,
  /\b(use local|move to|promote|split|separate|decouple)\b/i,
  /\b(improve|enhance|optimize) (code|structure|architecture)/i,
  /\b(remove duplicate|deduplicate)\b/i
];

const FIX_PATTERNS = [
  /\b(fix|correct|resolve|repair|patch)\b/i,
  /\b(bug|issue|error|crash|nil pointer|race condition|memory leak)\b/i,
  /\b(prevent|handle|guard against)\b/i
];

const FEAT_PATTERNS = [
  /\b(add|implement|introduce|create|new)\b/i,
  /\b(feature|capability|functionality|support for)\b/i,
  /\b(enable|allow|provide)\b/i
];

function parseArgs() {
  const args = process.argv.slice(2);
  const options = {
    count: 100,
    verbose: false,
    help: false
  };

  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (arg === '-n' || arg === '--count') {
      options.count = parseInt(args[++i], 10);
    } else if (arg === '-v' || arg === '--verbose') {
      options.verbose = true;
    } else if (arg === '-h' || arg === '--help') {
      options.help = true;
    }
  }

  return options;
}

function showHelp() {
  console.log(`
${BOLD}Check Commit Type Distribution${RESET}

Analyzes recent commits to report type distribution and identify potential misclassifications.

${BOLD}Usage:${RESET}
  node scripts/check-commit-type.mjs [options]

${BOLD}Options:${RESET}
  -n, --count <number>    Number of commits to analyze (default: 100)
  -v, --verbose           Show detailed analysis
  -h, --help              Show this help message

${BOLD}Examples:${RESET}
  node scripts/check-commit-type.mjs
  node scripts/check-commit-type.mjs -n 50
  node scripts/check-commit-type.mjs -n 200 -v
`);
}

function getCommits(count) {
  try {
    const output = execSync(`git log -${count} --pretty=format:"%H|%s"`, {
      encoding: 'utf-8',
      maxBuffer: 10 * 1024 * 1024
    });

    return output.trim().split('\n').map(line => {
      const [hash, message] = line.split('|');
      return { hash, message };
    });
  } catch (error) {
    console.error(`${COMMIT_TYPES.other.color}Error: Failed to fetch git commits${RESET}`);
    console.error(error.message);
    process.exit(1);
  }
}

function parseCommitType(message) {
  const match = message.match(/^(\w+)(?:\(([^)]+)\))?:\s*(.+)$/);

  if (!match) {
    return { type: 'other', scope: null, subject: message };
  }

  const [, type, scope, subject] = match;

  if (!COMMIT_TYPES[type]) {
    return { type: 'other', scope, subject };
  }

  return { type, scope, subject };
}

function analyzeCommit(commit) {
  const { type, scope, subject } = parseCommitType(commit.message);
  const suggestions = [];

  // Check for potential misclassifications
  if (type === 'feat') {
    if (REFACTOR_PATTERNS.some(pattern => pattern.test(subject))) {
      suggestions.push({
        current: 'feat',
        suggested: 'refactor',
        reason: 'Subject suggests code restructuring without new functionality'
      });
    }
  }

  if (type === 'fix') {
    if (FEAT_PATTERNS.some(pattern => pattern.test(subject)) &&
        !FIX_PATTERNS.some(pattern => pattern.test(subject))) {
      suggestions.push({
        current: 'fix',
        suggested: 'feat',
        reason: 'Subject suggests adding new functionality, not fixing a bug'
      });
    }
  }

  if (type === 'feat' && /\b(fix|correct|resolve)\b/i.test(subject)) {
    suggestions.push({
      current: 'feat',
      suggested: 'fix',
      reason: 'Subject suggests fixing incorrect behavior'
    });
  }

  return {
    ...commit,
    type,
    scope,
    subject,
    suggestions
  };
}

function calculateStats(commits) {
  const stats = {};

  for (const type of Object.keys(COMMIT_TYPES)) {
    stats[type] = 0;
  }

  for (const commit of commits) {
    stats[commit.type]++;
  }

  return stats;
}

function printStats(stats, total) {
  console.log(`\n${BOLD}Commit Type Distribution (last ${total} commits)${RESET}`);
  console.log('─'.repeat(60));

  const sortedTypes = Object.entries(stats)
    .filter(([type]) => type !== 'other')
    .sort((a, b) => b[1] - a[1]);

  for (const [type, count] of sortedTypes) {
    if (count === 0) continue;

    const percentage = ((count / total) * 100).toFixed(1);
    const bar = '█'.repeat(Math.round(count / total * 40));
    const { name, color } = COMMIT_TYPES[type];

    console.log(
      `${color}${type.padEnd(10)}${RESET} ${name.padEnd(15)} ` +
      `${count.toString().padStart(4)} (${percentage.padStart(5)}%) ${color}${bar}${RESET}`
    );
  }

  if (stats.other > 0) {
    const percentage = ((stats.other / total) * 100).toFixed(1);
    console.log(
      `${COMMIT_TYPES.other.color}other${RESET}     Other/Invalid   ` +
      `${stats.other.toString().padStart(4)} (${percentage.padStart(5)}%)`
    );
  }

  console.log('─'.repeat(60));
  console.log(`${BOLD}Total:${RESET} ${total} commits\n`);
}

function printRatioAnalysis(stats) {
  const feat = stats.feat || 0;
  const fix = stats.fix || 0;
  const refactor = stats.refactor || 0;

  console.log(`${BOLD}Ratio Analysis${RESET}`);
  console.log('─'.repeat(60));

  if (feat === 0 && fix === 0 && refactor === 0) {
    console.log('No feat, fix, or refactor commits found.');
    return;
  }

  // Calculate current ratio
  const gcd = (a, b) => b === 0 ? a : gcd(b, a % b);
  const divisor = [feat, fix, refactor].reduce((a, b) => gcd(a, b));

  const currentRatio = [
    Math.round(feat / divisor) || 0,
    Math.round(fix / divisor) || 0,
    Math.round(refactor / divisor) || 0
  ];

  console.log(`${BOLD}Current ratio (feat:fix:refactor):${RESET} ${currentRatio.join(':')}`);
  console.log(`${BOLD}Target ratio:${RESET}                      4:2:1`);

  // Calculate deviation from target
  const targetRatio = [4, 2, 1];
  const currentNormalized = [
    feat / (feat + fix + refactor),
    fix / (feat + fix + refactor),
    refactor / (feat + fix + refactor)
  ];
  const targetNormalized = [4/7, 2/7, 1/7];

  console.log('\n${BOLD}Recommendations:${RESET}');

  if (currentNormalized[0] > targetNormalized[0] * 1.2) {
    console.log(`  ${COMMIT_TYPES.feat.color}•${RESET} Reduce feat commits - ensure you're not labeling refactors as features`);
  }

  if (currentNormalized[2] < targetNormalized[2] * 0.8) {
    console.log(`  ${COMMIT_TYPES.refactor.color}•${RESET} Increase refactor commits - code improvements should be labeled as refactor`);
  }

  if (currentNormalized[1] < targetNormalized[1] * 0.8) {
    console.log(`  ${COMMIT_TYPES.fix.color}•${RESET} More bug fixes needed - or they might be mislabeled`);
  }

  console.log();
}

function printSuggestions(commits, verbose) {
  const commitsWithSuggestions = commits.filter(c => c.suggestions.length > 0);

  if (commitsWithSuggestions.length === 0) {
    console.log(`${BOLD}Potential Misclassifications${RESET}`);
    console.log('─'.repeat(60));
    console.log('✓ No obvious misclassifications detected.\n');
    return;
  }

  console.log(`${BOLD}Potential Misclassifications (${commitsWithSuggestions.length} found)${RESET}`);
  console.log('─'.repeat(60));

  const displayCount = verbose ? commitsWithSuggestions.length : Math.min(10, commitsWithSuggestions.length);

  for (let i = 0; i < displayCount; i++) {
    const commit = commitsWithSuggestions[i];
    const suggestion = commit.suggestions[0];

    console.log(`\n${DIM}${commit.hash.substring(0, 8)}${RESET} ${commit.message}`);
    console.log(
      `  ${COMMIT_TYPES[suggestion.current].color}${suggestion.current}${RESET} → ` +
      `${COMMIT_TYPES[suggestion.suggested].color}${suggestion.suggested}${RESET}: ` +
      `${DIM}${suggestion.reason}${RESET}`
    );
  }

  if (!verbose && commitsWithSuggestions.length > 10) {
    console.log(`\n${DIM}... and ${commitsWithSuggestions.length - 10} more. Use -v to see all.${RESET}`);
  }

  console.log();
}

function printInvalidCommits(commits) {
  const invalidCommits = commits.filter(c => c.type === 'other');

  if (invalidCommits.length === 0) {
    return;
  }

  console.log(`${BOLD}Invalid Commit Messages (${invalidCommits.length} found)${RESET}`);
  console.log('─'.repeat(60));
  console.log('These commits do not follow the conventional commit format:\n');

  for (const commit of invalidCommits.slice(0, 5)) {
    console.log(`${DIM}${commit.hash.substring(0, 8)}${RESET} ${commit.message}`);
  }

  if (invalidCommits.length > 5) {
    console.log(`${DIM}... and ${invalidCommits.length - 5} more.${RESET}`);
  }

  console.log();
}

function main() {
  const options = parseArgs();

  if (options.help) {
    showHelp();
    return;
  }

  console.log(`${BOLD}Analyzing last ${options.count} commits...${RESET}\n`);

  const commits = getCommits(options.count);
  const analyzedCommits = commits.map(analyzeCommit);
  const stats = calculateStats(analyzedCommits);

  printStats(stats, commits.length);
  printRatioAnalysis(stats);
  printSuggestions(analyzedCommits, options.verbose);
  printInvalidCommits(analyzedCommits);

  console.log(`${BOLD}Next Steps${RESET}`);
  console.log('─'.repeat(60));
  console.log('1. Review the suggestions above');
  console.log('2. Read the full guide: docs/COMMIT-CONVENTIONS.md');
  console.log('3. Use the commit template: git config commit.template .gitmessage');
  console.log('4. Run this check regularly to track progress\n');
}

main();

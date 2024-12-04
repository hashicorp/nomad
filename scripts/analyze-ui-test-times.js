#!/usr/bin/env node
/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

'use strict';
const fs = require('fs');
const path = require('path');

async function analyzeTestTimes() {
  const currentResults = JSON.parse(
    fs.readFileSync('combined-test-results.json')
  );

  // Create a map of test names to their durations
  const currentTestTimes = new Map();
  currentResults.tests.forEach(test => {
    currentTestTimes.set(test.name, test.duration);
  });

// Load and process historical results
console.log('[analyze-test-times] Processing historical results...\n');
const historicalAverages = new Map();
const historicalCounts = new Map();

// Read each historical result file
console.log('[analyze-test-times] Reading historical results directory...\n');
const historicalDir = 'historical-results';
const historicalFiles = fs.readdirSync(historicalDir)
  .filter(file => file.endsWith('.json'));

console.log(`[analyze-test-times] Found ${historicalFiles.length} JSON files`);

  // Debug: Show content of first file
  if (historicalFiles.length > 0) {
    const firstFile = fs.readFileSync(path.join(historicalDir, historicalFiles[0]), 'utf8');
    console.log('\n[analyze-test-times] First file content sample:');
    console.log(firstFile.substring(0, 500) + '...');
    console.log('\n[analyze-test-times] First file parsed:');
    const parsed = JSON.parse(firstFile);
    console.log(JSON.stringify(parsed, null, 2).substring(0, 500) + '...');
  }

historicalFiles.forEach((file, index) => {
  console.log(`[analyze-test-times] Reading ${file} (${index + 1} of ${historicalFiles.length})...`);
  try {
    const historical = JSON.parse(fs.readFileSync(path.join(historicalDir, file), 'utf8'));
    
    // Debug output
    console.log(`[analyze-test-times] File ${file}:`);
    console.log(`  - Has summary: ${!!historical.summary}`);
    if (historical.summary) {
      console.log(`  - Failed tests: ${historical.summary.failed}`);
      console.log(`  - Total tests: ${historical.summary.total}`);
    }
    console.log(`  - Has tests array: ${!!historical.tests}`);
    if (historical.tests) {
      console.log(`  - Number of tests: ${historical.tests.length}`);
    }

    if (historical.summary && historical.summary.failed === 0) {
      historical.tests.forEach(test => {
        const current = historicalAverages.get(test.name) || 0;
        const count = historicalCounts.get(test.name) || 0;
        historicalAverages.set(test.name, current + test.duration);
        historicalCounts.set(test.name, count + 1);
      });
    } else {
      console.log(`[analyze-test-times] Skipping ${file} because it has failed tests or invalid format`);
    }
  } catch (error) {
    console.log(`[analyze-test-times] Error processing ${file}:`, error.message);
  }
});

// Debug output after processing
console.log('\n[analyze-test-times] Processing complete');
console.log(`Total unique tests found: ${historicalAverages.size}`);
if (historicalAverages.size > 0) {
  console.log('Sample of processed tests:');
  let i = 0;
  for (const [name, total] of historicalAverages) {
    if (i++ >= 3) break;
    const count = historicalCounts.get(name);
    console.log(`- ${name}: ${total}ms total, ${count} samples`);
  }
}
  // Calculate averages and compare
  const analysis = {
    timestamp: new Date().toISOString(),
    sha: process.env.GITHUB_SHA,
    summary: {
      totalTests: currentResults.tests.length,
      significantlySlower: 0,
      significantlyFaster: 0
    },
    testComparisons: []
  };

  // In the analyzeTestTimes function:
  console.log('[analyze-test-times] Comparing current test times with historical averages...');
  currentTestTimes.forEach((currentDuration, testName) => {
    const totalHistorical = historicalAverages.get(testName) || 0;
    const count = historicalCounts.get(testName) || 1;
    const historicalAverage = totalHistorical / count;

    // Skip tests with no historical data
    if (historicalAverage === 0) {
      console.log(`[analyze-test-times] Skipping ${testName} - no historical data`);
      return;
    }

    // Consider a test significantly different if it's 25% slower/faster
    const percentDiff = ((currentDuration - historicalAverage) / historicalAverage) * 100;
    
    if (Math.abs(percentDiff) >= 25) {
      console.log(`[analyze-test-times] Found significant difference in ${testName}: ${percentDiff.toFixed(1)}% change`);
      analysis.testComparisons.push({
        name: testName,
        currentDuration,
        historicalAverage,
        percentDiff,  // This will now always be a number
        samples: count
      });

      if (percentDiff > 0) {
        analysis.summary.significantlySlower++;
      } else {
        analysis.summary.significantlyFaster++;
      }
    }
  });

  // Sort by most significant differences first
  analysis.testComparisons.sort((a, b) => Math.abs(b.percentDiff) - Math.abs(a.percentDiff));

  // Write analysis results
  fs.writeFileSync(
    'test-time-analysis.json',
    JSON.stringify(analysis, null, 2)
  );

  // Output summary to console for GitHub Actions
  console.log('\nTest Time Analysis Summary:');
  console.log(`Total Tests: ${analysis.summary.totalTests}`);
  console.log(`Significantly Slower: ${analysis.summary.significantlySlower}`);
  console.log(`Significantly Faster: ${analysis.summary.significantlyFaster}`);
  
  if (analysis.testComparisons.length > 0) {
    console.log('\nMost Significant Changes:');
    analysis.testComparisons.slice(0, 5).forEach(comp => {
      console.log(`${comp.name}:`);
      console.log(`  Current: ${comp.currentDuration}ms`);
      console.log(`  Historical Avg: ${comp.historicalAverage}ms`);
      console.log(`  Change: ${comp.percentDiff.toFixed(1)}%`);
    });
  }
}

if (require.main === module) {
  analyzeTestTimes();
}

module.exports = analyzeTestTimes;

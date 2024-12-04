#!/usr/bin/env node
/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

'use strict';
const fs = require('fs');

async function analyzeTestTimes() {
  const currentResults = JSON.parse(
    fs.readFileSync('../ui/combined-test-results.json')
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
  const historicalFiles = fs.readdirSync('../historical-results');
  historicalFiles.forEach((file, index) => {
    console.log(`[analyze-test-times] Reading ${file} (${index + 1} of ${historicalFiles.length})...`);
    const historical = JSON.parse(
      fs.readFileSync(`../historical-results/${file}`)
    );

    if (historical.summary.failed === 0) {
      historical.tests.forEach(test => {
        const current = historicalAverages.get(test.name) || 0;
        const count = historicalCounts.get(test.name) || 0;
        historicalAverages.set(test.name, current + test.duration);
        historicalCounts.set(test.name, count + 1);
      });
    } else {
      console.log(`[analyze-test-times] Skipping ${file} because it has failed tests`);
    }
  });

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

  currentTestTimes.forEach((currentDuration, testName) => {
    const totalHistorical = historicalAverages.get(testName) || 0;
    const count = historicalCounts.get(testName) || 1;
    const historicalAverage = totalHistorical / count;

    // Consider a test significantly different if it's 25% slower/faster
    const percentDiff = ((currentDuration - historicalAverage) / historicalAverage) * 100;
    
    if (Math.abs(percentDiff) >= 25) {
      analysis.testComparisons.push({
        name: testName,
        currentDuration,
        historicalAverage,
        percentDiff,
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
    '../ui/test-time-analysis.json',
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

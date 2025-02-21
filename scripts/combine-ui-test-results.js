#!/usr/bin/env node
/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

'use strict';
const fs = require('fs');

const NUM_PARTITIONS = 4;

function combineResults() {
  const results = [];
  let duration = 0;
  let aggregateSummary = { total: 0, passed: 0, failed: 0 };

  for (let i = 1; i <= NUM_PARTITIONS; i++) {
    try {
      const data = JSON.parse(
        fs.readFileSync(`../test-results/test-results-${i}/test-results.json`).toString()
      );
      results.push(...data.tests);
      duration += data.duration;
      aggregateSummary.total += data.summary.total;
      aggregateSummary.passed += data.summary.passed;
      aggregateSummary.failed += data.summary.failed;
    } catch (err) {
      console.error(`Error reading partition ${i}:`, err);
    }
  }

  const output = {
    timestamp: new Date().toISOString(),
    sha: process.env.GITHUB_SHA,
    summary: {
      total: aggregateSummary.total,
      passed: aggregateSummary.passed,
      failed: aggregateSummary.failed
    },
    duration,
    tests: results
  };

  fs.writeFileSync('../ui/combined-test-results.json', JSON.stringify(output, null, 2));
}

if (require.main === module) {
  combineResults();
}

module.exports = combineResults;

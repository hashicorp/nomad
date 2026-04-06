/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { get } from '@ember/object';
import d3Format from 'd3-format';

import { formatBytes, formatHertz } from 'nomad-ui/utils/units';

const formatPercent = d3Format.format('+.0%');
const sumAggregate = (total, val) => total + val;

export default class ResourcesDiffs {
  constructor(model, multiplier, recommendations, excludedRecommendations) {
    this.model = model;
    this.multiplier = multiplier;
    this.recommendations = recommendations;
    this.excludedRecommendations = excludedRecommendations.filter((r) =>
      recommendations.includes(r),
    );
  }

  get cpu() {
    const included = this.includedRecommendations.filter(item => get(item, 'resource') === 'CPU');
    const excluded = this.excludedRecommendations.filter(item => get(item, 'resource') === 'CPU');

    return new ResourceDiffs(
      this.model.reservedCPU,
      'reservedCPU',
      'MHz',
      this.multiplier,
      included,
      excluded,
    );
  }

  get memory() {
    const included = this.includedRecommendations.filter(item => get(item, 'resource') === 'MemoryMB');
    const excluded = this.excludedRecommendations.filter(item => get(item, 'resource') === 'MemoryMB');

    return new ResourceDiffs(
      this.model.reservedMemory,
      'reservedMemory',
      'MiB',
      this.multiplier,
      included,
      excluded,
    );
  }

  get includedRecommendations() {
    return this.recommendations.filter(function(...args) { return !((r) =>
      this.excludedRecommendations.includes(r)).apply(this, args); },
    );
  }
}

class ResourceDiffs {
  constructor(
    base,
    baseTaskPropertyName,
    units,
    multiplier,
    includedRecommendations,
    excludedRecommendations,
  ) {
    this.base = base;
    this.baseTaskPropertyName = baseTaskPropertyName;
    this.units = units;
    this.multiplier = multiplier;
    this.included = includedRecommendations;
    this.excluded = excludedRecommendations;
  }

  get recommended() {
    if (this.included.length) {
      return (
        this.included.map(item => get(item, 'value')).reduce(sumAggregate, 0) +
        this.excluded
          .map(item => get(item, `task.${this.baseTaskPropertyName}`))
          .reduce(sumAggregate, 0)
      );
    } else {
      return this.base;
    }
  }

  get delta() {
    return this.recommended - this.base;
  }

  get aggregateDiff() {
    return this.delta * this.multiplier;
  }

  get absoluteAggregateDiff() {
    const delta = Math.abs(this.aggregateDiff);

    if (this.units === 'MiB') {
      return formatBytes(delta, 'MiB');
    } else if (this.units === 'MHz') {
      return formatHertz(delta, 'MHz');
    } else {
      return `${delta} ${this.units}`;
    }
  }

  get signedDiff() {
    const delta = this.aggregateDiff;
    return `${signForDelta(delta)}${this.absoluteAggregateDiff}`;
  }

  get percentDiff() {
    return formatPercent(this.delta / this.base);
  }
}

function signForDelta(delta) {
  if (delta > 0) {
    return '+';
  } else if (delta < 0) {
    return '-';
  }

  return '';
}

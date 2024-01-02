/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import ResourcesDiffs from 'nomad-ui/utils/resources-diffs';

export default class DasResourceTotalsComponent extends Component {
  get diffs() {
    return new ResourcesDiffs(
      this.args.model,
      1,
      this.args.recommendations,
      this.args.excludedRecommendations
    );
  }

  get cpuClass() {
    return classForDelta(this.diffs.cpu.delta);
  }

  get memoryClass() {
    return classForDelta(this.diffs.memory.delta);
  }
}

function classForDelta(delta) {
  if (delta > 0) {
    return 'increase';
  } else if (delta < 0) {
    return 'decrease';
  }

  return '';
}

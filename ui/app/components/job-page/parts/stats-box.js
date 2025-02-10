/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

export default class StatsBox extends Component {
  @service system;

  get packDetails() {
    let packMeta = this.args.job?.meta?.structured.root.children.pack;
    if (!packMeta) {
      return null;
    } else {
      return packMeta.files
        .map((file) => {
          return {
            key: file.name,
            value: file.variable.value,
          };
        })
        .reduce((acc, file) => {
          acc[file.key] = file.value;
          return acc;
        }, {});
    }
  }
}

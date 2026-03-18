/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { concat, hash } from '@ember/helper';
import Component from '@glimmer/component';
import SafeLinkTo from 'nomad-ui/components/safe-link-to';

export default class SortBy extends Component {
  get isActive() {
    return this.args.currentProp === this.args.prop;
  }

  get shouldSortDescending() {
    return !this.isActive || !this.args.sortDescending;
  }

  <template>
    <th
      class={{concat
        (if @class (concat @class " ") "")
        "is-selectable "
        (if this.isActive "is-active " "")
        (if @sortDescending "desc" "asc")
      }}
      title={{@title}}
    >
      <SafeLinkTo
        @query={{hash
          sortProperty=@prop
          sortDescending=this.shouldSortDescending
        }}
        data-test-sort-by={{@prop}}
      >
        {{yield}}
      </SafeLinkTo>
    </th>
  </template>
}

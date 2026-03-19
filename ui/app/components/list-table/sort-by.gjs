/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { LinkTo } from '@ember/routing';
import { concat, hash } from '@ember/helper';

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
      <LinkTo
        @query={{hash
          sortProperty=@prop
          sortDescending=this.shouldSortDescending
        }}
        data-test-sort-by={{@prop}}
      >
        {{yield}}
      </LinkTo>
    </th>
  </template>
}

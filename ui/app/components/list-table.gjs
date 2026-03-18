/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { hash, concat } from '@ember/helper';
import { or } from 'ember-truth-helpers';
import ListTableSortBy from 'nomad-ui/components/list-table/sort-by';
import ListTableTableBody from 'nomad-ui/components/list-table/table-body';
import ListTableTableHead from 'nomad-ui/components/list-table/table-head';

export default class ListTable extends Component {
  // Plan for a future with metadata (e.g., isSelected)
  get decoratedSource() {
    return (this.args.source || []).map((row) => ({
      model: row,
    }));
  }

  <template>
    <table class={{concat "table " (or @class "")}}>
      {{yield
        (hash
          head=ListTableTableHead
          body=(component ListTableTableBody rows=this.decoratedSource)
          sortBy=(component
            ListTableSortBy
            currentProp=@sortProperty
            sortDescending=@sortDescending
          )
        )
      }}
    </table>
  </template>
}

/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

const TableBody = <template>
  <tbody class={{@class}}>
    {{#each @rows as |row index|}}
      {{yield row index}}
    {{/each}}
  </tbody>
</template>;

export default TableBody;

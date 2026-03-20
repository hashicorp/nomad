/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

const TableHead = <template>
  <thead class={{@class}} ...attributes>
    <tr>
      {{yield}}
    </tr>
  </thead>
</template>;

export default TableHead;

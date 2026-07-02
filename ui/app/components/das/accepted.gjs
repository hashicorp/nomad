/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { HdsIcon } from '@hashicorp/design-system-components/components';

export const DasAccepted = <template>
  <section class="das-accepted">
    <main>
      <h3>Recommendation accepted</h3>
      <p>A new version of this job will now be deployed.</p>
    </main>
    <HdsIcon @name="check-circle-fill" />
  </section>
</template>;

export default DasAccepted;

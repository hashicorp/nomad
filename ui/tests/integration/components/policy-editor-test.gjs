/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import PolicyEditor from 'nomad-ui/components/policy-editor';

module('Integration | Component | policy-editor', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function (assert) {
    await render(<template><PolicyEditor /></template>);
    await componentA11yAudit(this.element, assert);
  });

  test('Only has editable name if new', async function (assert) {
    const newMockPolicy = {
      isNew: true,
      name: 'New Policy',
    };

    const oldMockPolicy = {
      isNew: false,
      name: 'Old Policy',
    };

    await render(
      <template><PolicyEditor @policy={{newMockPolicy}} /></template>,
    );
    assert.dom('[data-test-policy-name-input]').exists();

    await render(
      <template><PolicyEditor @policy={{oldMockPolicy}} /></template>,
    );
    assert.dom('[data-test-policy-name-input]').doesNotExist();
  });
});

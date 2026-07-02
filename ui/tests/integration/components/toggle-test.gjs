/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { find, render } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import sinon from 'sinon';
import { create } from 'ember-cli-page-object';
import ToggleComponent from 'nomad-ui/components/toggle';
import togglePageObject from 'nomad-ui/tests/pages/components/toggle';

const Toggle = create(togglePageObject());

module('Integration | Component | toggle', function (hooks) {
  setupRenderingTest(hooks);

  const commonProperties = () => ({
    isActive: false,
    isDisabled: false,
    label: 'Label',
    onToggle: sinon.spy(),
  });

  const renderToggle = async (props) => {
    await render(
      <template>
        <ToggleComponent
          @isActive={{props.isActive}}
          @isDisabled={{props.isDisabled}}
          @onToggle={{props.onToggle}}
        >
          {{props.label}}
        </ToggleComponent>
      </template>,
    );
  };

  test('presents as a label with an inner checkbox and display span, and text', async function (assert) {
    const props = commonProperties();

    await renderToggle(props);

    assert.deepEqual(
      Toggle.label,
      props.label,
      `Label should be ${props.label}`,
    );
    assert.ok(Toggle.isPresent);
    assert.notOk(Toggle.isActive);
    assert.ok(find('[data-test-toggler]'));
    assert.deepEqual(
      find('[data-test-input]').tagName.toLowerCase(),
      'input',
      'The input is a real HTML input',
    );
    assert.deepEqual(
      find('[data-test-input]').getAttribute('type'),
      'checkbox',
      'The input type is checkbox',
    );

    await componentA11yAudit(this.element, assert);
  });

  test('the isActive property dictates the active state and class', async function (assert) {
    const props = commonProperties();

    await renderToggle(props);

    assert.notOk(Toggle.isActive);
    assert.notOk(Toggle.hasActiveClass);

    await renderToggle({
      ...props,
      isActive: true,
    });

    assert.ok(Toggle.isActive);
    assert.ok(Toggle.hasActiveClass);

    await componentA11yAudit(this.element, assert);
  });

  test('the isDisabled property dictates the disabled state and class', async function (assert) {
    const props = commonProperties();

    await renderToggle(props);

    assert.notOk(Toggle.isDisabled);
    assert.notOk(Toggle.hasDisabledClass);

    await renderToggle({
      ...props,
      isDisabled: true,
    });

    assert.ok(Toggle.isDisabled);
    assert.ok(Toggle.hasDisabledClass);

    await componentA11yAudit(this.element, assert);
  });

  test('toggling the input calls the onToggle action', async function (assert) {
    const props = commonProperties();

    await renderToggle(props);

    await Toggle.toggle();
    assert.deepEqual(props.onToggle.callCount, 1);
  });
});

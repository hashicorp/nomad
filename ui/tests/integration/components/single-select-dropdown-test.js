/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { findAll, find, render } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import {
  selectChoose,
  clickTrigger,
} from 'ember-power-select/test-support/helpers';
import sinon from 'sinon';
import hbs from 'htmlbars-inline-precompile';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module('Integration | Component | single-select dropdown', function (hooks) {
  setupRenderingTest(hooks);

  const commonProperties = () => ({
    label: 'Type',
    selection: 'nomad',
    options: [
      { key: 'consul', label: 'Consul' },
      { key: 'nomad', label: 'Nomad' },
      { key: 'terraform', label: 'Terraform' },
      { key: 'packer', label: 'Packer' },
      { key: 'vagrant', label: 'Vagrant' },
      { key: 'vault', label: 'Vault' },
    ],
    onSelect: sinon.spy(),
  });

  const commonTemplate = hbs`
    <SingleSelectDropdown
      @label={{this.label}}
      @options={{this.options}}
      @selection={{this.selection}}
      @onSelect={{this.onSelect}} />
  `;

  test('component shows label and selection in the trigger', async function (assert) {
    assert.expect(4);

    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    assert.ok(
      find('.ember-power-select-trigger').textContent.includes(props.label)
    );
    assert.ok(
      find('.ember-power-select-trigger').textContent.includes(
        props.options.findBy('key', props.selection).label
      )
    );
    assert.notOk(find('[data-test-dropdown-options]'));

    await componentA11yAudit(this.element, assert);
  });

  test('all options are shown in the dropdown', async function (assert) {
    assert.expect(7);

    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await clickTrigger('[data-test-single-select-dropdown]');

    assert.equal(
      findAll('.ember-power-select-option').length,
      props.options.length,
      'All options are shown'
    );
    findAll('.ember-power-select-option').forEach((optionEl, index) => {
      assert.equal(
        optionEl.querySelector('.dropdown-label').textContent.trim(),
        props.options[index].label
      );
    });
  });

  test('selecting an option calls `onSelect` with the key for the selected option', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    const option = props.options.findBy('key', 'terraform');
    await selectChoose('[data-test-single-select-dropdown]', option.label);

    assert.ok(props.onSelect.calledWith(option.key));
  });
});

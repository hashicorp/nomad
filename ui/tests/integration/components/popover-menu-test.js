/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { click, render } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import { create } from 'ember-cli-page-object';
import popoverMenuPageObject from 'nomad-ui/tests/pages/components/popover-menu';

const PopoverMenu = create(popoverMenuPageObject());

module('Integration | Component | popover-menu', function (hooks) {
  setupRenderingTest(hooks);

  const commonProperties = (overrides) =>
    Object.assign(
      {
        triggerClass: '',
        label: 'Trigger Label',
      },
      overrides
    );

  const commonTemplate = hbs`
    <PopoverMenu
      @isOpen={{isOpen}}
      @label={{label}}
      @triggerClass={{triggerClass}} as |m|>
      <h1>This is a heading</h1>
      <label>This is an input: <input id="mock-input-for-test" type="text" /></label>
      <button id="mock-button-for-test" type="button" onclick={{action m.actions.close}}>Close Button</button>
    </PopoverMenu>
  `;

  test('presents as a button with a chevron-down icon', async function (assert) {
    assert.expect(5);

    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    assert.ok(PopoverMenu.isPresent);
    assert.ok(PopoverMenu.labelHasIcon);
    assert.notOk(PopoverMenu.menu.isOpen);
    assert.equal(PopoverMenu.label, props.label);
    await componentA11yAudit(this.element, assert);
  });

  test('clicking the trigger button toggles the popover menu', async function (assert) {
    assert.expect(3);

    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);
    assert.notOk(PopoverMenu.menu.isOpen);

    await PopoverMenu.toggle();

    assert.ok(PopoverMenu.menu.isOpen);
    await componentA11yAudit(this.element, assert);
  });

  test('the trigger gets the triggerClass prop assigned as a class', async function (assert) {
    const specialClass = 'is-special';
    const props = commonProperties({ triggerClass: specialClass });
    this.setProperties(props);
    await render(commonTemplate);

    assert.dom('[data-test-popover-trigger]').hasClass('is-special');
  });

  test('pressing DOWN ARROW when the trigger is focused opens the popover menu', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);
    assert.notOk(PopoverMenu.menu.isOpen);

    await PopoverMenu.focus();
    await PopoverMenu.downArrow();

    assert.ok(PopoverMenu.menu.isOpen);
  });

  test('pressing TAB when the trigger button is focused and the menu is open focuses the first focusable element in the popover menu', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await PopoverMenu.focus();
    await PopoverMenu.downArrow();

    assert.dom('[data-test-popover-trigger]').isFocused();

    await PopoverMenu.focusNext();

    assert.dom('#mock-input-for-test').isFocused();
  });

  test('pressing ESC when the popover menu is open closes the menu and returns focus to the trigger button', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await PopoverMenu.toggle();
    assert.ok(PopoverMenu.menu.isOpen);

    await PopoverMenu.esc();

    assert.notOk(PopoverMenu.menu.isOpen);
  });

  test('the ember-basic-dropdown object is yielded as context, including the close action', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await PopoverMenu.toggle();
    assert.ok(PopoverMenu.menu.isOpen);

    await click('#mock-button-for-test');
    assert.notOk(PopoverMenu.menu.isOpen);
  });
});

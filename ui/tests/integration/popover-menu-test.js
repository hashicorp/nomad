import { find, click } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import { create } from 'ember-cli-page-object';
import popoverMenuPageObject from 'nomad-ui/tests/pages/components/popover-menu';

const PopoverMenu = create(popoverMenuPageObject());

module('Integration | Component | popover-menu', function(hooks) {
  setupRenderingTest(hooks);

  const commonProperties = overrides =>
    Object.assign(
      {
        triggerClass: '',
        label: 'Trigger Label',
      },
      overrides
    );

  const commonTemplate = hbs`
    {{#popover-menu
      isOpen=isOpen
      label=label
      triggerClass=triggerClass as |m|}}
      <h1>This is a heading</h1>
      <label>This is an input: <input id="mock-input-for-test" type="text" /></label>
      <button id="mock-button-for-test" type="button" onclick={{action m.actions.close}}>Close Button</button>
    {{/popover-menu}}
  `;

  test('presents as a button with a chevron-down icon', async function(assert) {
    const props = commonProperties();
    this.setProperties(props);
    await this.render(commonTemplate);

    assert.ok(PopoverMenu.isPresent);
    assert.ok(PopoverMenu.labelHasIcon);
    assert.notOk(PopoverMenu.menu.isOpen);
    assert.equal(PopoverMenu.label, props.label);
  });

  test('clicking the trigger button toggles the popover menu', async function(assert) {
    const props = commonProperties();
    this.setProperties(props);
    await this.render(commonTemplate);
    assert.notOk(PopoverMenu.menu.isOpen);

    await PopoverMenu.toggle();

    assert.ok(PopoverMenu.menu.isOpen);
  });

  test('the trigger gets the triggerClass prop assigned as a class', async function(assert) {
    const specialClass = 'is-special';
    const props = commonProperties({ triggerClass: specialClass });
    this.setProperties(props);
    await this.render(commonTemplate);

    assert.ok(Array.from(find('[data-test-popover-trigger]').classList).includes('is-special'));
  });

  test('pressing DOWN ARROW when the trigger is focused opens the popover menu', async function(assert) {
    const props = commonProperties();
    this.setProperties(props);
    await this.render(commonTemplate);
    assert.notOk(PopoverMenu.menu.isOpen);

    await PopoverMenu.focus();
    await PopoverMenu.downArrow();

    assert.ok(PopoverMenu.menu.isOpen);
  });

  test('pressing TAB when the trigger button is focused and the menu is open focuses the first focusable element in the popover menu', async function(assert) {
    const props = commonProperties();
    this.setProperties(props);
    await this.render(commonTemplate);

    await PopoverMenu.focus();
    await PopoverMenu.downArrow();

    assert.equal(document.activeElement, find('[data-test-popover-trigger]'));

    await PopoverMenu.focusNext();

    assert.equal(document.activeElement, find('#mock-input-for-test'));
  });

  test('pressing ESC when the popover menu is open closes the menu and returns focus to the trigger button', async function(assert) {
    const props = commonProperties();
    this.setProperties(props);
    await this.render(commonTemplate);

    await PopoverMenu.toggle();
    assert.ok(PopoverMenu.menu.isOpen);

    await PopoverMenu.esc();

    assert.notOk(PopoverMenu.menu.isOpen);
  });

  test('the ember-basic-dropdown object is yielded as context, including the close action', async function(assert) {
    const props = commonProperties();
    this.setProperties(props);
    await this.render(commonTemplate);

    await PopoverMenu.toggle();
    assert.ok(PopoverMenu.menu.isOpen);

    await click('#mock-button-for-test');
    assert.notOk(PopoverMenu.menu.isOpen);
  });
});

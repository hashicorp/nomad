import { findAll, find, click, focus, keyEvent } from 'ember-native-dom-helpers';
import { moduleForComponent, test } from 'ember-qunit';
import sinon from 'sinon';
import wait from 'ember-test-helpers/wait';
import hbs from 'htmlbars-inline-precompile';

const TAB = 9;
const ESC = 27;
const SPACE = 32;
const ARROW_UP = 38;
const ARROW_DOWN = 40;

moduleForComponent('multi-select-dropdown', 'Integration | Component | multi-select dropdown', {
  integration: true,
});

const commonProperties = () => ({
  label: 'This is the dropdown label',
  selection: [],
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
  {{multi-select-dropdown
    label=label
    options=options
    selection=selection
    onSelect=onSelect}}
`;

test('component is initially closed', function(assert) {
  const props = commonProperties();
  this.setProperties(props);
  this.render(commonTemplate);

  assert.ok(find('.dropdown-trigger'), 'Trigger is shown');
  assert.equal(
    find('[data-test-dropdown-trigger]').textContent.trim(),
    props.label,
    'Trigger is appropriately labeled'
  );
  assert.notOk(find('[data-test-dropdown-options]'), 'Options are not rendered');
});

test('component opens the options dropdown when clicked', function(assert) {
  const props = commonProperties();
  this.setProperties(props);
  this.render(commonTemplate);

  click('[data-test-dropdown-trigger]');

  return wait()
    .then(() => {
      assert.ok(find('[data-test-dropdown-options]'), 'Options are shown now');
      click('[data-test-dropdown-trigger]');
      return wait();
    })
    .then(() => {
      assert.notOk(find('[data-test-dropdown-options]'), 'Options are hidden after clicking again');
    });
});

test('all options are shown in the options dropdown, each with a checkbox input', function(assert) {
  const props = commonProperties();
  this.setProperties(props);
  this.render(commonTemplate);

  click('[data-test-dropdown-trigger]');

  return wait().then(() => {
    assert.equal(
      findAll('[data-test-dropdown-option]').length,
      props.options.length,
      'All options are shown'
    );
    findAll('[data-test-dropdown-option]').forEach((optionEl, index) => {
      const label = props.options[index].label;
      assert.equal(optionEl.textContent.trim(), label, `Correct label for ${label}`);
      assert.ok(optionEl.querySelector('input[type="checkbox"]'), 'Option contains a checkbox');
    });
  });
});

test('onSelect gets called when an option is clicked', function(assert) {
  const props = commonProperties();
  this.setProperties(props);
  this.render(commonTemplate);

  click('[data-test-dropdown-trigger]');

  return wait()
    .then(() => {
      click('[data-test-dropdown-option] label');
      return wait();
    })
    .then(() => {
      assert.ok(props.onSelect.called, 'onSelect was called');
      const newSelection = props.onSelect.getCall(0).args[0];
      assert.deepEqual(
        newSelection,
        [props.options[0].key],
        'onSelect was called with the first option key'
      );
    });
});

test('the component trigger shows the selection count when there is a selection', function(assert) {
  const props = commonProperties();
  props.selection = [props.options[0].key, props.options[1].key];
  this.setProperties(props);
  this.render(commonTemplate);

  assert.ok(find('[data-test-dropdown-trigger] [data-test-dropdown-count]'), 'The count is shown');
  assert.equal(
    find('[data-test-dropdown-trigger] [data-test-dropdown-count]').textContent,
    props.selection.length,
    'The count is accurate'
  );

  this.set('selection', []);

  return wait().then(() => {
    assert.notOk(
      find('[data-test-dropdown-trigger] [data-test-dropdown-count]'),
      'The count is no longer shown when the selection is empty'
    );
  });
});

test('pressing DOWN when the trigger has focus opens the options list', function(assert) {
  const props = commonProperties();
  this.setProperties(props);
  this.render(commonTemplate);

  focus('[data-test-dropdown-trigger]');
  assert.notOk(find('[data-test-dropdown-options]'), 'Options are not shown on focus');
  keyEvent('[data-test-dropdown-trigger]', 'keydown', ARROW_DOWN);
  assert.ok(find('[data-test-dropdown-options]'), 'Options are now shown');
  assert.equal(
    document.activeElement,
    find('[data-test-dropdown-trigger]'),
    'The dropdown trigger maintains focus'
  );
});

test('pressing DOWN when the trigger has focus and the options list is open focuses the first option', function(assert) {
  const props = commonProperties();
  this.setProperties(props);
  this.render(commonTemplate);

  focus('[data-test-dropdown-trigger]');
  keyEvent('[data-test-dropdown-trigger]', 'keydown', ARROW_DOWN);
  keyEvent('[data-test-dropdown-trigger]', 'keydown', ARROW_DOWN);
  assert.equal(
    document.activeElement,
    find('[data-test-dropdown-option]'),
    'The first option now has focus'
  );
});

test('pressing TAB when the trigger has focus and the options list is open focuses the first option', function(assert) {
  const props = commonProperties();
  this.setProperties(props);
  this.render(commonTemplate);

  focus('[data-test-dropdown-trigger]');
  keyEvent('[data-test-dropdown-trigger]', 'keydown', ARROW_DOWN);
  keyEvent('[data-test-dropdown-trigger]', 'keydown', TAB);
  assert.equal(
    document.activeElement,
    find('[data-test-dropdown-option]'),
    'The first option now has focus'
  );
});

test('pressing UP when the first list option is focused does nothing', function(assert) {
  const props = commonProperties();
  this.setProperties(props);
  this.render(commonTemplate);

  click('[data-test-dropdown-trigger]');

  focus('[data-test-dropdown-option]');
  keyEvent('[data-test-dropdown-option]', 'keydown', ARROW_UP);
  assert.equal(
    document.activeElement,
    find('[data-test-dropdown-option]'),
    'The first option maintains focus'
  );
});

test('pressing DOWN when the a list option is focused moves focus to the next list option', function(assert) {
  const props = commonProperties();
  this.setProperties(props);
  this.render(commonTemplate);

  click('[data-test-dropdown-trigger]');

  focus('[data-test-dropdown-option]');
  keyEvent('[data-test-dropdown-option]', 'keydown', ARROW_DOWN);
  assert.equal(
    document.activeElement,
    findAll('[data-test-dropdown-option]')[1],
    'The second option has focus'
  );
});

test('pressing DOWN when the last list option has focus does nothing', function(assert) {
  const props = commonProperties();
  this.setProperties(props);
  this.render(commonTemplate);

  click('[data-test-dropdown-trigger]');

  focus('[data-test-dropdown-option]');
  const optionEls = findAll('[data-test-dropdown-option]');
  const lastIndex = optionEls.length - 1;
  optionEls.forEach((option, index) => {
    keyEvent(option, 'keydown', ARROW_DOWN);
    if (index < lastIndex) {
      assert.equal(document.activeElement, optionEls[index + 1], `Option ${index + 1} has focus`);
    }
  });

  keyEvent(optionEls[lastIndex], 'keydown', ARROW_DOWN);
  assert.equal(document.activeElement, optionEls[lastIndex], `Option ${lastIndex} still has focus`);
});

test('onSelect gets called when pressing SPACE when a list option is focused', function(assert) {
  const props = commonProperties();
  this.setProperties(props);
  this.render(commonTemplate);

  click('[data-test-dropdown-trigger]');

  focus('[data-test-dropdown-option]');
  keyEvent('[data-test-dropdown-option]', 'keydown', SPACE);

  assert.ok(props.onSelect.called, 'onSelect was called');
  const newSelection = props.onSelect.getCall(0).args[0];
  assert.deepEqual(
    newSelection,
    [props.options[0].key],
    'onSelect was called with the first option key'
  );
});

test('list options have a positive tabindex and are therefore sequentially navigable', function(assert) {
  const props = commonProperties();
  this.setProperties(props);
  this.render(commonTemplate);

  click('[data-test-dropdown-trigger]');

  findAll('[data-test-dropdown-option]').forEach(option => {
    assert.ok(parseInt(option.getAttribute('tabindex'), 10) > 0, 'tabindex is a positive value');
  });
});

test('the checkboxes inside list options have a negative tabindex and are therefore not sequentially navigable', function(assert) {
  const props = commonProperties();
  this.setProperties(props);
  this.render(commonTemplate);

  click('[data-test-dropdown-trigger]');

  findAll('[data-test-dropdown-option]').forEach(option => {
    assert.ok(
      parseInt(option.querySelector('input[type="checkbox"]').getAttribute('tabindex'), 10) < 0,
      'tabindex is a negative value'
    );
  });
});

test('pressing ESC when the options list is open closes the list and returns focus to the dropdown trigger', function(assert) {
  const props = commonProperties();
  this.setProperties(props);
  this.render(commonTemplate);

  focus('[data-test-dropdown-trigger]');
  keyEvent('[data-test-dropdown-trigger]', 'keydown', ARROW_DOWN);
  keyEvent('[data-test-dropdown-trigger]', 'keydown', ARROW_DOWN);
  keyEvent('[data-test-dropdown-option]', 'keydown', ESC);

  assert.notOk(find('[data-test-dropdown-options]'), 'The options list is hidden once more');
  assert.equal(
    document.activeElement,
    find('[data-test-dropdown-trigger]'),
    'The trigger has focus'
  );
});

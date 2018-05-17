import { find, findAll } from 'ember-native-dom-helpers';
import { test, moduleForComponent } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import flat from 'npm:flat';

const { flatten } = flat;

moduleForComponent('attributes-table', 'Integration | Component | attributes table', {
  integration: true,
});

const commonAttributes = {
  key: 'value',
  nested: {
    props: 'are',
    supported: 'just',
    fine: null,
  },
  so: {
    are: {
      deeply: {
        nested: 'properties',
        like: 'these ones',
      },
    },
  },
};

test('should render a row for each key/value pair in a deep object', function(assert) {
  this.set('attributes', commonAttributes);
  this.render(hbs`{{attributes-table attributes=attributes}}`);

  const rowsCount = Object.keys(flatten(commonAttributes)).length;
  assert.equal(
    this.$('[data-test-attributes-section]').has('[data-test-value]').length,
    rowsCount,
    `Table has ${rowsCount} rows with values`
  );
});

test('should render the full path of key/value pair from the root of the object', function(assert) {
  this.set('attributes', commonAttributes);
  this.render(hbs`{{attributes-table attributes=attributes}}`);

  assert.equal(find('[data-test-key]').textContent.trim(), 'key', 'Row renders the key');
  assert.equal(find('[data-test-value]').textContent.trim(), 'value', 'Row renders the value');

  const deepRow = findAll('[data-test-attributes-section]')[8];
  assert.equal(
    deepRow.querySelector('[data-test-key]').textContent.trim(),
    'so.are.deeply.nested',
    'Complex row renders the full path to the key'
  );
  assert.equal(
    deepRow.querySelector('[data-test-prefix]').textContent.trim(),
    'so.are.deeply.',
    'The prefix is faded to put emphasis on the attribute'
  );
  assert.equal(deepRow.querySelector('[data-test-value]').textContent.trim(), 'properties');
});

test('should render a row for key/value pairs even when the value is another object', function(
  assert
) {
  this.set('attributes', commonAttributes);
  this.render(hbs`{{attributes-table attributes=attributes}}`);

  const countOfParentRows = countOfParentKeys(commonAttributes);
  assert.equal(
    findAll('[data-test-heading]').length,
    countOfParentRows,
    'Each key for a nested object gets a row with no value'
  );
});

function countOfParentKeys(obj) {
  return Object.keys(obj).reduce((count, key) => {
    const value = obj[key];
    return isObject(value) ? count + 1 + countOfParentKeys(value) : count;
  }, 0);
}

function isObject(value) {
  return !Array.isArray(value) && value != null && typeof value === 'object';
}

import { findAll } from 'ember-native-dom-helpers';
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
    this.$('tbody tr').has('td:eq(1)').length,
    rowsCount,
    `Table has ${rowsCount} rows with values`
  );
});

test('should render the full path of key/value pair from the root of the object', function(assert) {
  this.set('attributes', commonAttributes);
  this.render(hbs`{{attributes-table attributes=attributes}}`);

  assert.equal(
    this.$('tbody tr:eq(0) td')
      .get(0)
      .textContent.trim(),
    'key',
    'Simple row renders only the key'
  );
  assert.equal(
    this.$('tbody tr:eq(0) td')
      .get(1)
      .textContent.trim(),
    'value'
  );

  assert.equal(
    this.$('tbody tr:eq(8) td')
      .get(0)
      .textContent.trim(),
    'so.are.deeply.nested',
    'Complex row renders the full path to the key'
  );
  assert.equal(
    this.$('tbody tr:eq(8) td:eq(0) .is-faded')
      .get(0)
      .textContent.trim(),
    'so.are.deeply.',
    'The prefix is faded to put emphasis on the attribute'
  );
  assert.equal(
    this.$('tbody tr:eq(8) td')
      .get(1)
      .textContent.trim(),
    'properties'
  );
});

test('should render a row for key/value pairs even when the value is another object', function(
  assert
) {
  this.set('attributes', commonAttributes);
  this.render(hbs`{{attributes-table attributes=attributes}}`);

  const countOfParentRows = countOfParentKeys(commonAttributes);
  assert.equal(
    findAll('tbody tr td[colspan="2"]').length,
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

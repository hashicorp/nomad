import { findAll, find } from 'ember-native-dom-helpers';
import { test, moduleForComponent } from 'ember-qunit';
import { faker } from 'ember-cli-mirage';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('breadcrumb-trail', 'Integration | Component | breadcrumb trail', {
  integration: true,
});


test('component renders correctly', function(assert) {
  const fixtures = createFixtures();
  this.set(
    'items',
    fixtures
  );
  this.render(hbs`
    {{breadcrumb-trail items=items}}
  `);

  assert.equal(findAll('nav').length, 1, 'nav exists');
  assert.equal(findAll('a').length, fixtures.length, `There are ${fixtures.length} breadcrumbs`);
  assert.equal(find('[data-test-breadcrumb="0"]').textContent, fixtures[0].label, 'The first breadcrumb has the correct label');
});
function createFixtures() {
  return [
    {
      label: faker.random.word,
      params: [
        0
      ]
    },
    {
      label: faker.random.word,
      params: [
        1
      ]
    }
  ];
}


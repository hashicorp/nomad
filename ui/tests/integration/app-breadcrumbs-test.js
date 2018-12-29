import Service from '@ember/service';
import { getOwner } from '@ember/application';
import RSVP from 'rsvp';
import { test, moduleForComponent } from 'ember-qunit';
import { findAll } from 'ember-native-dom-helpers';
import wait from 'ember-test-helpers/wait';
import hbs from 'htmlbars-inline-precompile';
import PromiseObject from 'nomad-ui/utils/classes/promise-object';

moduleForComponent('app-breadcrumbs', 'Integration | Component | app breadcrumbs', {
  integration: true,
  beforeEach() {
    const mockBreadcrumbs = Service.extend({
      breadcrumbs: [],
    });

    this.register('service:breadcrumbs', mockBreadcrumbs);
    this.breadcrumbs = getOwner(this).lookup('service:breadcrumbs');
  },
});

const commonCrumbs = [{ label: 'One', args: ['one'] }, { label: 'Two', args: ['two'] }];

const template = hbs`
  {{app-breadcrumbs}}
`;

test('breadcrumbs comes from the breadcrumbs service', function(assert) {
  this.breadcrumbs.set('breadcrumbs', commonCrumbs);

  this.render(template);

  assert.equal(
    findAll('[data-test-breadcrumb]').length,
    commonCrumbs.length,
    'The number of crumbs matches the crumbs from the service'
  );
});

test('every breadcrumb is rendered correctly', function(assert) {
  this.breadcrumbs.set('breadcrumbs', commonCrumbs);

  this.render(template);

  const renderedCrumbs = findAll('[data-test-breadcrumb]');

  renderedCrumbs.forEach((crumb, index) => {
    assert.equal(
      crumb.textContent.trim(),
      commonCrumbs[index].label,
      `Crumb ${index} is ${commonCrumbs[index].label}`
    );
  });
});

test('when breadcrumbs are pending promises, an ellipsis is rendered', function(assert) {
  let resolvePromise;
  const promise = new RSVP.Promise(resolve => {
    resolvePromise = resolve;
  });

  this.breadcrumbs.set('breadcrumbs', [
    { label: 'One', args: ['one'] },
    PromiseObject.create({ promise }),
    { label: 'Three', args: ['three'] },
  ]);

  this.render(template);

  assert.equal(
    findAll('[data-test-breadcrumb]')[1].textContent.trim(),
    '…',
    'Promise breadcrumb is in a loading state'
  );

  resolvePromise({ label: 'Two', args: ['two'] });

  return wait().then(() => {
    assert.equal(
      findAll('[data-test-breadcrumb]')[1].textContent.trim(),
      'Two',
      'Promise breadcrumb has resolved and now renders Two'
    );
  });
});

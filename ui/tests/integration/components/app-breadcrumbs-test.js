import Service from '@ember/service';
import RSVP from 'rsvp';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { findAll, render, settled } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import PromiseObject from 'nomad-ui/utils/classes/promise-object';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module('Integration | Component | app breadcrumbs', function(hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function() {
    const mockBreadcrumbs = Service.extend({
      init() {
        this._super(...arguments);
        this.breadcrumbs = [];
      },
    });

    this.owner.register('service:breadcrumbs', mockBreadcrumbs);
    this.breadcrumbs = this.owner.lookup('service:breadcrumbs');
  });

  const commonCrumbs = [{ label: 'One', args: ['one'] }, { label: 'Two', args: ['two'] }];

  const template = hbs`
    <ul><AppBreadcrumbs /></ul>
  `;

  test('breadcrumbs comes from the breadcrumbs service', async function(assert) {
    this.breadcrumbs.set('breadcrumbs', commonCrumbs);

    await render(template);

    assert.equal(
      findAll('[data-test-breadcrumb]').length,
      commonCrumbs.length,
      'The number of crumbs matches the crumbs from the service'
    );
  });

  test('every breadcrumb is rendered correctly', async function(assert) {
    this.breadcrumbs.set('breadcrumbs', commonCrumbs);

    await render(template);

    const renderedCrumbs = findAll('[data-test-breadcrumb]');

    renderedCrumbs.forEach((crumb, index) => {
      assert.equal(
        crumb.textContent.trim(),
        commonCrumbs[index].label,
        `Crumb ${index} is ${commonCrumbs[index].label}`
      );
    });
  });

  test('when breadcrumbs are pending promises, an ellipsis is rendered', async function(assert) {
    let resolvePromise;
    const promise = new RSVP.Promise(resolve => {
      resolvePromise = resolve;
    });

    this.breadcrumbs.set('breadcrumbs', [
      { label: 'One', args: ['one'] },
      PromiseObject.create({ promise }),
      { label: 'Three', args: ['three'] },
    ]);

    await render(template);

    assert.equal(
      findAll('[data-test-breadcrumb]')[1].textContent.trim(),
      '…',
      'Promise breadcrumb is in a loading state'
    );

    await componentA11yAudit(this.element, assert);

    resolvePromise({ label: 'Two', args: ['two'] });

    return settled().then(() => {
      assert.equal(
        findAll('[data-test-breadcrumb]')[1].textContent.trim(),
        'Two',
        'Promise breadcrumb has resolved and now renders Two'
      );
    });
  });
});

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { find, findAll, render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';

module('Integration | Component | job-page/parts/body', function(hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function() {
    window.localStorage.clear();
    this.server = startMirage();
    this.server.createList('namespace', 3);
  });

  hooks.afterEach(function() {
    this.server.shutdown();
    window.localStorage.clear();
  });

  test('includes a subnav for the job', async function(assert) {
    this.set('job', {});

    await render(hbs`
      {{#job-page/parts/body job=job}}
        <div class="inner-content">Inner content</div>
      {{/job-page/parts/body}}
    `);

    assert.ok(find('[data-test-subnav="job"]'), 'Job subnav is rendered');
  });

  test('the subnav includes the deployments link when the job is a service', async function(assert) {
    const store = this.owner.lookup('service:store');
    const job = await store.createRecord('job', {
      id: 'service-job',
      type: 'service',
    });

    this.set('job', job);

    await render(hbs`
      {{#job-page/parts/body job=job}}
        <div class="inner-content">Inner content</div>
      {{/job-page/parts/body}}
    `);

    const subnavLabels = findAll('[data-test-tab]').map(anchor => anchor.textContent);
    assert.ok(subnavLabels.some(label => label === 'Definition'), 'Definition link');
    assert.ok(subnavLabels.some(label => label === 'Versions'), 'Versions link');
    assert.ok(subnavLabels.some(label => label === 'Deployments'), 'Deployments link');
  });

  test('the subnav does not include the deployments link when the job is not a service', async function(assert) {
    const store = this.owner.lookup('service:store');
    const job = await store.createRecord('job', {
      id: 'batch-job',
      type: 'batch',
    });

    this.set('job', job);

    await render(hbs`
      {{#job-page/parts/body job=job}}
        <div class="inner-content">Inner content</div>
      {{/job-page/parts/body}}
    `);

    const subnavLabels = findAll('[data-test-tab]').map(anchor => anchor.textContent);
    assert.ok(subnavLabels.some(label => label === 'Definition'), 'Definition link');
    assert.ok(subnavLabels.some(label => label === 'Versions'), 'Versions link');
    assert.notOk(subnavLabels.some(label => label === 'Deployments'), 'Deployments link');
  });

  test('body yields content to a section after the subnav', async function(assert) {
    this.set('job', {});

    await render(hbs`
      {{#job-page/parts/body job=job}}
        <div class="inner-content">Inner content</div>
      {{/job-page/parts/body}}
    `);

    assert.ok(
      find('[data-test-subnav="job"] + .section > .inner-content'),
      'Content is rendered immediately after the subnav'
    );
  });
});

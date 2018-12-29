import { run } from '@ember/runloop';
import { getOwner } from '@ember/application';
import { test, moduleForComponent } from 'ember-qunit';
import { find, findAll } from 'ember-native-dom-helpers';
import wait from 'ember-test-helpers/wait';
import hbs from 'htmlbars-inline-precompile';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';

moduleForComponent('job-page/parts/body', 'Integration | Component | job-page/parts/body', {
  integration: true,
  beforeEach() {
    window.localStorage.clear();
    this.server = startMirage();
    this.server.createList('namespace', 3);
  },
  afterEach() {
    this.server.shutdown();
    window.localStorage.clear();
  },
});

test('includes a subnav for the job', function(assert) {
  this.set('job', {});

  this.render(hbs`
    {{#job-page/parts/body job=job}}
      <div class="inner-content">Inner content</div>
    {{/job-page/parts/body}}
  `);

  return wait().then(() => {
    assert.ok(find('[data-test-subnav="job"]'), 'Job subnav is rendered');
  });
});

test('the subnav includes the deployments link when the job is a service', function(assert) {
  const store = getOwner(this).lookup('service:store');
  let job;

  run(() => {
    job = store.createRecord('job', {
      id: 'service-job',
      type: 'service',
    });
  });

  this.set('job', job);

  this.render(hbs`
    {{#job-page/parts/body job=job}}
      <div class="inner-content">Inner content</div>
    {{/job-page/parts/body}}
  `);

  return wait().then(() => {
    const subnavLabels = findAll('[data-test-tab]').map(anchor => anchor.textContent);
    assert.ok(subnavLabels.some(label => label === 'Definition'), 'Definition link');
    assert.ok(subnavLabels.some(label => label === 'Versions'), 'Versions link');
    assert.ok(subnavLabels.some(label => label === 'Deployments'), 'Deployments link');
  });
});

test('the subnav does not include the deployments link when the job is not a service', function(assert) {
  const store = getOwner(this).lookup('service:store');
  let job;

  run(() => {
    job = store.createRecord('job', {
      id: 'batch-job',
      type: 'batch',
    });
  });

  this.set('job', job);

  this.render(hbs`
    {{#job-page/parts/body job=job}}
      <div class="inner-content">Inner content</div>
    {{/job-page/parts/body}}
  `);

  return wait().then(() => {
    const subnavLabels = findAll('[data-test-tab]').map(anchor => anchor.textContent);
    assert.ok(subnavLabels.some(label => label === 'Definition'), 'Definition link');
    assert.ok(subnavLabels.some(label => label === 'Versions'), 'Versions link');
    assert.notOk(subnavLabels.some(label => label === 'Deployments'), 'Deployments link');
  });
});

test('body yields content to a section after the subnav', function(assert) {
  this.set('job', {});

  this.render(hbs`
    {{#job-page/parts/body job=job}}
      <div class="inner-content">Inner content</div>
    {{/job-page/parts/body}}
  `);

  return wait().then(() => {
    assert.ok(
      find('[data-test-subnav="job"] + .section > .inner-content'),
      'Content is rendered immediately after the subnav'
    );
  });
});

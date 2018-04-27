import { getOwner } from '@ember/application';
import { test, moduleForComponent } from 'ember-qunit';
import wait from 'ember-test-helpers/wait';
import hbs from 'htmlbars-inline-precompile';
import generateResources from '../../mirage/data/generate-resources';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import Response from 'ember-cli-mirage/response';

moduleForComponent('allocation-row', 'Integration | Component | allocation row', {
  integration: true,
  beforeEach() {
    this.store = getOwner(this).lookup('service:store');
    this.server = startMirage();
    this.server.create('namespace');
    this.server.create('node');
    this.server.create('job', { createAllocations: false });
  },
  afterEach() {
    this.server.shutdown();
  },
});

test('Allocation row polls for stats, even when it errors or has an invalid response', function(assert) {
  const component = this;

  let currentFrame = 0;
  let frames = [
    JSON.stringify({ ResourceUsage: generateResources() }),
    JSON.stringify({ ResourceUsage: generateResources() }),
    null,
    '<Not>Valid JSON</Not>',
    JSON.stringify({ ResourceUsage: generateResources() }),
  ];
  const backoffSequence = [50];

  this.server.get('/client/allocation/:id/stats', function() {
    const response = frames[++currentFrame];

    // Disable polling to stop the EC task in the component
    if (currentFrame >= frames.length) {
      component.set('enablePolling', false);
    }

    if (response) {
      return response;
    }
    return new Response(500, {}, '');
  });

  this.server.create('allocation');
  this.store.findAll('allocation');

  let allocation;

  return wait()
    .then(() => {
      allocation = this.store.peekAll('allocation').get('firstObject');

      this.setProperties({
        allocation,
        backoffSequence,
        context: 'job',
        enablePolling: true,
      });

      this.render(hbs`
        {{allocation-row
          allocation=allocation
          context=context
          backoffSequence=backoffSequence
          enablePolling=enablePolling}}
      `);
      return wait();
    })
    .then(() => {
      assert.equal(
        this.server.pretender.handledRequests.filterBy(
          'url',
          `/v1/client/allocation/${allocation.get('id')}/stats`
        ).length,
        frames.length,
        'Requests continue to be made after malformed responses and server errors'
      );
    });
});

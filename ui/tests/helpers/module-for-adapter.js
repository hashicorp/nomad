import { module } from 'qunit';
import { setupTest } from 'ember-qunit';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';

export default function(modelName, description) {
  // moduleForModel correctly creates the store service
  // but moduleFor does not.
  module(description, function(hooks) {
    setupTest(hooks);
    this.store = this.owner.lookup('service:store');

    // Initializers don't run automatically in unit tests
    fragmentSerializerInitializer(this.owner);

    // Reassign the subject to provide the adapter
    this.subject = () => this.store.adapterFor(modelName);
  });
}

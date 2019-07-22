import { Factory, trait } from 'ember-cli-mirage';
import { generateResources } from '../common';

export default Factory.extend({
  name: () => '!!!this should be set by the allocation that owns this task state!!!',

  resources: generateResources,

  withReservedv4Ports: trait({
    resources: () => generateResources({ networks: { minPorts: 1 } }),
  }),

  withReservedv6Ports: trait({
    resources: () => generateResources({ networks: { ipv6: true, minPorts: 1 } }),
  }),

  withoutReservedPorts: trait({
    resources: () => generateResources({ networks: { minPorts: 0, maxPorts: 0 } }),
  }),
});

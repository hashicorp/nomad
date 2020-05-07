import { alias } from '@ember/object/computed';
import AllocationRow from 'nomad-ui/components/allocation-row';

export default AllocationRow.extend({
  pluginAllocation: null,
  allocation: alias('pluginAllocation.allocation'),
});

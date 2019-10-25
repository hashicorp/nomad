import Watchable from './watchable';
import addToPath from 'nomad-ui/utils/add-to-path';

export default Watchable.extend({
  setEligible(node) {
    return this.setEligibility(node, true);
  },

  setIneligible(node) {
    return this.setEligibility(node, false);
  },

  setEligibility(node, isEligible) {
    const url = addToPath(this.urlForFindRecord(node.id, 'node'), '/eligibility');
    return this.ajax(url, 'POST', {
      data: {
        NodeID: node.id,
        Eligibility: isEligible ? 'eligible' : 'ineligible',
      },
    });
  },
});

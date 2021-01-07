import { create, text, visitable } from 'ember-cli-page-object';

import TopoViz from 'nomad-ui/tests/pages/components/topo-viz';
import notification from 'nomad-ui/tests/pages/components/notification';

export default create({
  visit: visitable('/topology'),

  infoPanelTitle: text('[data-test-info-panel-title]'),
  filteredNodesWarning: notification('[data-test-filtered-nodes-warning]'),

  viz: TopoViz('[data-test-topo-viz]'),
});

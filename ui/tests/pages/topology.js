import { create, text, visitable } from 'ember-cli-page-object';

import TopoViz from 'nomad-ui/tests/pages/components/topo-viz';

export default create({
  visit: visitable('/topology'),

  infoPanelTitle: text('[data-test-info-panel-title]'),

  viz: TopoViz('[data-test-topo-viz]'),
});

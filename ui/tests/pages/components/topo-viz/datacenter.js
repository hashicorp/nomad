import { collection, text } from 'ember-cli-page-object';
import TopoVizNode from './node';

export default (scope) => ({
  scope,

  label: text('[data-test-topo-viz-datacenter-label]'),
  nodes: collection('[data-test-topo-viz-node]', TopoVizNode()),
});

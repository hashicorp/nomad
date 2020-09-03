import Component from '@glimmer/component';

export default class TopoViz extends Component {
  get datacenters() {
    const datacentersMap = this.args.nodes.reduce((datacenters, node) => {
      if (!datacenters[node.datacenter]) datacenters[node.datacenter] = [];
      datacenters[node.datacenter].push(node);
      return datacenters;
    }, {});

    return Object.keys(datacentersMap)
      .map(key => ({ name: key, nodes: datacentersMap[key] }))
      .sortBy('name');
  }
}

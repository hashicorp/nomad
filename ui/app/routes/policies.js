import Route from '@ember/routing/route';

import notifyError from 'nomad-ui/utils/notify-error';
import PathTree from 'nomad-ui/utils/path-tree';

export default class PoliciesRoute extends Route {
  async model() {
    try {
      // await this.store.findAll('policies');
      // const policies = await this.store.query(
      //   'policy',
      //   { },
      //   { reload: true }
      // );

      const policies = [
        {
          id: 1,
          name: 'foo',
          description: 'bar',
          rules: `
            foo = "bar"
          `,
        },
      ];

      return {
        policies,
        pathTree: new PathTree(policies),
      };
    } catch (e) {
      notifyError(this)(e);
    }
  }
}

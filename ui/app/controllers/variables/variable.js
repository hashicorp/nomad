import Controller from '@ember/controller';

export default class VariablesVariableController extends Controller {
  get breadcrumbs() {
    let crumbs = [];
    this.model.path.split('/').reduce((m, n) => {
      crumbs.push({
        label: n,
        args:
          m + n === this.model.path // If the last crumb, link to the var itself
            ? [`variables.variable`, m + n]
            : [`variables.path`, m + n],
      });
      return m + n + '/';
    }, []);
    return crumbs;
  }
}

import Controller from '@ember/controller';

export default class VariablesPathController extends Controller {
  get breadcrumbs() {
    let crumbs = [];
    this.model.absolutePath.split('/').reduce((m, n) => {
      crumbs.push({
        label: n,
        args: [`variables.path`, m + n],
      });
      return m + n + '/';
    }, []);
    return crumbs;
  }
}

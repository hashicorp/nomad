import Controller from '@ember/controller';

export default class RunController extends Controller {
  onSubmit(id, namespace) {
    this.transitionToRoute('jobs.job', id, {
      queryParams: { jobNamespace: namespace },
    });
  }
}

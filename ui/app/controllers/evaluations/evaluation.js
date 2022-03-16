import Controller from '@ember/controller';

export default class EvaluationController extends Controller {
  get eval() {
    return this.model;
  }

  get breadcrumb() {
    return {
      title: 'Evaluation',
      label: this.eval.get('shortId'),
      args: ['evaluations.evaluation', this.eval.get('id')],
    };
  }
}

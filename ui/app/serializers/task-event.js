import ApplicationSerializer from './application';

export default class TaskEventSerializer extends ApplicationSerializer {
  attrs = {
    message: 'DisplayMessage',
  };

  separateNanos = ['Time'];
}

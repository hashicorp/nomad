import ApplicationAdapter from './application';

export default class VarAdapter extends ApplicationAdapter {
  handleResponse(_status, _headers, response) {
    response.forEach((item) => (item.ID = item.Path));
    return response;
  }

  // methodForRequest(requestType, id, snapshot) {
  //   console.log('req', requestType);
  //   if (requestType === 'updateRecord') {
  //     return 'PUT';
  //   }
  //   return super.methodForRequest(...arguments);
  // }
}

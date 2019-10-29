/* eslint-env node */
import hbs from 'htmlbars-inline-precompile';
<%= importMD %>

export default {
  title: '<%= classifiedModuleName %>',
  parameters: {
    notes
  }
};

export const <%= classifiedModuleName %> = () => {
  return {
    template: hbs`
      <h5 class="title is-5"><%= header %></h5>
      <<%= classifiedModuleName %>/>
    `,
    context: {},
  }
};

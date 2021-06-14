var markedpp = require('markedpp'),
    md = '!include(jobs.mdx)',
    options = { dirname: '../../website/content/api-docs' };

markedpp(md, function(err, result){
    console.log(result);
    /* Outputs
    <!-- !numberedheadings -->

    <!-- !toc (level=1) -->

    * [1\. hello](#1-hello)

    <!-- toc! -->

    # 1\. hello

    ## 1.1\. hello again
    */
});
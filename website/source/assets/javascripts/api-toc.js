// This is adapted from Terraform’s web site:
// https://github.com/hashicorp/terraform-website/blob/b218a3a9aac14462065e2035e8281d38e784af47/content/source/assets/javascripts/application.js

document.addEventListener('turbolinks:load', function() {
    "use strict";

    // On docs/content pages, add a hierarchical quick nav menu if there are
    // more than two H2 headers.
    var headers = $('#inner').find('h2');
    if (headers.length > 2 && $("div#inner-quicknav").length === 0) {
        // Build the quick-nav HTML:
        $("#inner h1").first().after(
            '<div id="inner-quicknav">' +
                '<span id="inner-quicknav-trigger" class="g-type-label">' +
                    'Jump to Section' +
                    '<svg width="9" height="5" xmlns="http://www.w3.org/2000/svg"><path d="M8.811 1.067a.612.612 0 0 0 0-.884.655.655 0 0 0-.908 0L4.5 3.491 1.097.183a.655.655 0 0 0-.909 0 .615.615 0 0 0 0 .884l3.857 3.75a.655.655 0 0 0 .91 0l3.856-3.75z" fill-rule="evenodd"/></svg>' +
                '</span>' +
                '<ul class="dropdown"></ul>' +
            '</div>'
        );
        var quickNav = $('#inner-quicknav > ul.dropdown');
        headers.each(function(index, element) {
            var level = element.nodeName.toLowerCase();
            var header_text = $(element).text();
            var header_id = $(element).attr('id');
            quickNav.append('<li class="level-' + level + '"><a href="#' + header_id + '">' + header_text + '</a></li>');
        });
        // Attach event listeners:
        // Trigger opens and closes.
        $('#inner-quicknav #inner-quicknav-trigger').on('click', function(e) {
            $(this).siblings('ul').toggleClass('active');
            e.stopPropagation();
        });
        // Clicking inside the quick-nav doesn't close it.
        quickNav.on('click', function(e) {
            e.stopPropagation();
        });
        // Jumping to a section means you're done with the quick-nav.
        quickNav.find('li a').on('click', function() {
            quickNav.removeClass('active');
        });
        // Clicking outside the quick-nav closes it.
        $('body').on('click', function() {
            quickNav.removeClass('active');
        });
    }
});

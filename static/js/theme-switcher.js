/*!
 * Theme Switcher
 *
 * Pico.css - https://picocss.com
 * Copyright 2019 - Licensed under MIT
 */

(function() {

  /**
   * Config
   */

  var switcher = {
    button: {
      element:    'BUTTON',
      class:      'contrast switcher theme-switcher',
      on:         '<i>Turn on dark mode</i>',
      off:        '<i>Turn off dark mode</i>'
    },
    target:       'body', // Button append in target
    selector:     'button.theme-switcher',  // Button selector in Dom
    currentTheme: systemColorScheme()
  };



  /**
   * Init
   */

  themeSwitcher();



  /**
   * Get System Color Scheme
   *
   * @return {string}
   */

  function systemColorScheme() {
    // Check `theme` cookie first.
    let theme = getCookie("theme");
    switch (theme) {
      case "light":
      case "dark":
      return theme;
    }

    if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
      return 'dark';
    }
    else {
      return 'light';
    }
  }



  /**
   * Display Theme Switcher
   */

  function themeSwitcher() {

    // Insert Switcher
    var button = document.createElement(switcher.button.element);
    button.className = switcher.button.class;
    document.querySelector(switcher.target).appendChild(button);

    // Set Current Theme
    setTheme(switcher.currentTheme);

    // Click Listener on Switcher
    var switchers = document.querySelectorAll(switcher.selector);
    for (var i = 0; i < switchers.length; i++) {
      switchers[i].addEventListener('click', function(event) {

        // Switch Theme
        if (switcher.currentTheme == 'light') {
          setTheme('dark');
          storeTheme('dark');
        }
        else {
          setTheme('light');
          storeTheme('light');
        }

      }, false);
    }
  }

  function getCookie(cname) {
    var name = cname + "=";
    var ca = document.cookie.split(';');
    for(var i = 0; i < ca.length; i++) {
      var c = ca[i];
      while (c.charAt(0) == ' ') {
        c = c.substring(1);
      }
      if (c.indexOf(name) == 0) {
        return c.substring(name.length, c.length);
      }
    }
    return "";
  }

  /**
   * Set Theme
   *
   * @param {string} set
   */

  function setTheme(set) {

    // Text toggle
    if (set == 'light') {
      var label = switcher.button.on;
    }
    else {
      var label = switcher.button.off;
    }

    // Apply theme
    document.querySelector('html').setAttribute('data-theme', set);
    var switchers = document.querySelectorAll(switcher.selector);
    for (var i = 0; i < switchers.length; i++) {
      switchers[i].innerHTML = label;
      switchers[i].setAttribute('aria-label', stripTags(label));
    }
    switcher.currentTheme = set;
  }

  /**
   * Store Theme - Persists the theme in a cookie
   *
   * @param {string} set
   */

  function storeTheme(set) {
    // Set a cookie to persist the theme
    document.cookie = "theme=" + set + "; expires=Fri, 31 Dec 9999 23:59:59 UTC; path=/";
  }


  /**
   * Strip tags
   *
   * @param {string} html
   * @return {string}
   */

  function stripTags(html) {
    return html.replace(/<[^>]*>?/gm, '');
  }

})();

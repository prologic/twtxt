// Creare's 'Implied Consent' EU Cookie Law Banner v:2.4
// Conceived by Robert Kent, James Bavington & Tom Foyster

var dropCookie = true;                      // false disables the Cookie, allowing you to style the banner
var cookieDuration = 14;                    // Number of days before the cookie expires, and the banner reappears
var cookieName = 'complianceCookie';        // Name of our cookie
var cookieValue = 'on';                     // Value of cookie

function createDiv(){
    u("body").prepend('<div id="cookie-law" class="container-fluid"><p>This website uses cookies. By continuing we assume your permission to deploy cookies, as detailed in our <a href="/privacy" rel="nofollow" title="Privacy Policy">privacy policy</a>. <a role="button" href="javascript:void(0);" onclick="removeMe();">Close</a></p></div>');
    createCookie(window.cookieName,window.cookieValue, window.cookieDuration); // Create the cookie
}


function createCookie(name,value,days) {
    if (days) {
        var date = new Date();
        date.setTime(date.getTime()+(days*24*60*60*1000));
        var expires = "; expires="+date.toGMTString();
    }
    else var expires = "";
    if(window.dropCookie) {
        document.cookie = name+"="+value+expires+"; path=/";
    }
}

function checkCookie(name) {
    var nameEQ = name + "=";
    var ca = document.cookie.split(';');
    for(var i=0;i < ca.length;i++) {
        var c = ca[i];
        while (c.charAt(0)==' ') c = c.substring(1,c.length);
        if (c.indexOf(nameEQ) == 0) return c.substring(nameEQ.length,c.length);
    }
    return null;
}

function eraseCookie(name) {
    createCookie(name,"",-1);
}

window.onload = function(){
    if(checkCookie(window.cookieName) != window.cookieValue){
        createDiv();
    }
}

function removeMe(){
	var element = document.getElementById('cookie-law');
	element.parentNode.removeChild(element);
}

function replyTo(e) {
  e.preventDefault();

  var el = u("textarea#text")
  var text = document.getElementById("text");

  el.empty();
  el.text(u(e.target).data("reply"));
  el.scroll();

  text.focus();

  var size = el.text().length;

  text.setSelectionRange(size, size);
}

function editTwt(e) {
  e.preventDefault();

  var el = u("textarea#text")
  var text = document.getElementById("text");

  el.empty();
  el.text(u(e.target).data("text"));
  el.scroll();

  text.focus();

  var size = el.text().length;

  text.setSelectionRange(size, size);

  u("#replaceTwt").first().value = u(e.target).data("hash");
}

function deleteTwt(e) {
  e.preventDefault();

  if (confirm("Are you sure you want to delete this twt? This cannot be undone!")) {
    Twix.ajax({
      type: "DELETE",
      url: u("#twtForm").attr("action"),
      success: function(data) {
        var hash = u(e.target).data("hash");
        u("#" + hash).remove();
      }
    });
  }
};

u(".reply").on("click", replyTo);
u(".edit").on("click", editTwt);
u(".delete").on("click", deleteTwt);

u("#post").on("click", function(e) {
  e.preventDefault();
  u("#post").html("<i class=\"icss-spinner icss-pulse\"></i>&nbsp;Posting...");
  u("#post").attr("disabled", true);
  u("#twtForm").first().submit();
});

u.prototype.getSelection = function() {
  var e = this.first();

  return (
    /* mozilla / dom 3.0 */
    ('selectionStart' in e && function() {
      var l = e.selectionEnd - e.selectionStart;
      return { start: e.selectionStart, end: e.selectionEnd, length: l, text: e.value.substr(e.selectionStart, l) }; }) ||
    /* exploder */
    (document.selection && function() {
      e.focus();

      var r = document.selection.createRange();
      if (r === null) {
        return { start: 0, end: e.value.length, length: 0 }
      }

      var re = e.createTextRange();
      var rc = re.duplicate();
      re.moveToBookmark(r.getBookmark());
      rc.setEndPoint('EndToStart', re);

      return { start: rc.text.length, end: rc.text.length + r.text.length, length: r.text.length, text: r.text };
  }) ||
    /* browser not supported */
    function() { return null; }
  )();
}

u.prototype.replaceSelection = function() {
  var e = this.first();

  var text = arguments[0] || '';

  return (
    /* mozilla / dom 3.0 */
    ('selectionStart' in e && function() {
      e.value = e.value.substr(0, e.selectionStart) + text + e.value.substr(e.selectionEnd, e.value.length);
      return this;
    }) ||
      /* exploder */
      (document.selection && function() {
        e.focus();
        document.selection.createRange().text = text;
        return this;
    }) ||
      /* browser not supported */
      function() {
        e.value += text;
        return jQuery(e);
      }
  )();
}

function formatText(selector, fmt) {
    var finalText = "";

    var selectedText = selector.getSelection().text;

    if (selectedText.length == 0) {
        finalText = fmt + fmt;
    } else {
        finalText = fmt + selectedText + fmt;
    }

    selector.replaceSelection(finalText , true);
    selector.first().focus();
    if(!selectedText.length) {
      selector.first().selectionEnd = selector.first().value.length - fmt.length;
    }
}

function insertText(selector, text) {
  selector.replaceSelection(selector.text() + text, true);
  selector.scroll();
  selector.first().focus();
  selector.first().setSelectionRange(-1 ,-1);
  selector.first().selectionEnd = selector.first().value.length - 1
}

u('#bBtn').on("click", function(e) {
  e.preventDefault();
  formatText(u("textarea#text"), "**");
});

u('#iBtn').on("click", function(e) {
  e.preventDefault();
  formatText(u("textarea#text"), "*");
});

u('#sBtn').on("click", function(e) {
  e.preventDefault();
  formatText(u("textarea#text"), "~~");
});


u('#cBtn').on("click", function(e) {
  e.preventDefault();
  formatText(u("textarea#text"), "`");
});

u('#lnkBtn').on("click", function(e) {
  e.preventDefault();
  insertText(u("textarea#text"), "[title](https://)");
});

u('#imgBtn').on("click", function(e) {
  e.preventDefault();
  insertText(u("textarea#text"), "![](https://)");
});

u('#uploadMedia').on("change", function(e){
    u("#uploadMediaButton").removeClass("icss-camera");
    u("#uploadMediaButton").addClass("icss-spinner icss-pulse");

    u("#uploadMedia").html("<i class=\"icss-spinner icss-pulse\"></i>");
    Twix.ajax({
      type: "POST",
      url: u("#uploadForm").attr("action"),
      data: new FormData(u("#uploadForm").first()),
      success: function(data) {
        var el = u("textarea#text")
        var text = document.getElementById("text");

        text.value += " ![](" + data.Path + ") ";
        el.scroll();
        text.focus();

        var size = el.text().length;
        text.setSelectionRange(size, size);

        u("#uploadMediaButton").removeClass("icss-spinner icss-pulse");
        u("#uploadMediaButton").addClass("icss-camera");
      },
      error: function (statusCode, statusText) {
        u("#uploadMediaButton").removeClass("icss-spinner icss-pulse");
        u("#uploadMediaButton").addClass("icss-camera");
        alert("An error occurred uploading your media: " + statusCode + " " + statusText);
      }
    });
});

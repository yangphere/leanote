/**
 * @file 提示帮助
 * @author  life
 * 
 */
define('tips', [], function() {
    var tpl = ['<div class="modal fade bs-modal-sm" tabindex="-1" role="dialog" aria-labelledby="mySmallModalLabel" aria-hidden="true">',
            '<div class="modal-dialog modal-sm">',
                '<div class="modal-content">',
                    '<div class="modal-header">',
                        '<button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="' + getMsg('close') + '"></button>',
                        '<h4 class="modal-title" class="modalTitle">' + getMsg('editorTips') + '</h4>',
                    '</div>',
                    '<div class="modal-body">' + getMsg('editorTipsInfo') + '</div>',
                    '<div class="modal-footer">',
                        '<button type="button" class="btn btn-secondary" data-bs-dismiss="modal">' + getMsg('close') + '</button>',
                    '</div>',
                '</div>',
            '</div>',
       '</div>'].join('');
    var $tpl = $(tpl);

    var view = {
        init: function () {
            $("#tipsBtn").on('click', function() {
                showBootstrapModal($tpl[0]);
            });
        }
    };

    view.init();
});

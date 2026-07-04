package ui



import (

	"fmt"

	"reader-manager/logger"

	"reader-manager/model"

	"reader-manager/store"

	"time"



	"fyne.io/fyne/v2"

	"fyne.io/fyne/v2/container"

	"fyne.io/fyne/v2/dialog"

	"fyne.io/fyne/v2/layout"

	"fyne.io/fyne/v2/storage"

	"fyne.io/fyne/v2/widget"

)



// mainWindowState 持有主窗口需要引用的可变组件

type mainWindowState struct {

	store      *store.Store

	logger     *logger.Logger

	statusLabel *widget.Label

	table      *widget.Table

}



// BuildMainWindow 构建主窗口：工具栏 + 查询区 + 表格

func BuildMainWindow(w fyne.Window, st *store.Store, l *logger.Logger) {

	state := &mainWindowState{store: st, logger: l}



	// ============== 顶部工具栏 ==============

	tools := container.NewHBox(

		makeToolButton("新增", "➕", func() { showReaderDialog(w, st) }),

		makeToolButton("修改", "✏", nil),

		makeToolButton("删除", "➖", nil),

		makeToolButton("续期", "➕", nil),

		makeToolButton("补卡", "♻", nil),

		makeToolButton("类型", "⇆", nil),

		makeToolButton("导入", "⇥", nil),

		makeToolButton("导出", "⇤", func() { exportData(w, st) }),

		makeToolButton("打印", "🖨", nil),

		makeToolButton("备份", "📥", func() { backupData(w, st) }),

		makeToolButton("恢复", "📤", func() { restoreData(w, st, state) }),

		makeToolButton("清除", "🗑", func() { clearData(w, st, state) }),

		makeToolButton("关闭", "⏻", func() { w.Close() }),

	)



	// ============== 顶部说明文字 ==============

	notice := widget.NewRichTextFromMarkdown(

		"**说明：** 红色字体显示的读者，表示已经挂失。\n蓝色字体显示的读者，表示证件到期！",

	)

	notice.Wrapping = fyne.TextWrapOff

	noticeBox := container.NewHBox(layout.NewSpacer(), notice, layout.NewSpacer())



	// ============== 查询区 ==============

	queryWay := widget.NewSelect([]string{"任意方式", "读者编号", "姓名", "身份证号", "联系电话"}, nil)

	queryWay.SetSelected("任意方式")



	queryInput := widget.NewEntry()

	queryInput.SetPlaceHolder("输入读者编号或姓名")



	showExpired := widget.NewCheck("显示过期读者", nil)



	moreQuery := widget.NewButton("更多条件查询[F8]", func() {

		dialog.ShowInformation("提示", "更多条件查询", w)

	})



	queryBox := container.NewVBox(

		container.NewHBox(

			widget.NewLabel("选择查询方式:"), queryWay,

			layout.NewSpacer(),

			widget.NewLabel("输入读者信息查询[F3]:"), queryInput,

			layout.NewSpacer(),

			showExpired, moreQuery,

		),

	)



	// ============== 表格 ==============

	headers := model.Headers()



	tbl := widget.NewTable(

		func() (int, int) {

			// 行数 = 数据行 + 1（表头）

			return st.Count() + 1, len(headers)

		},

		func() fyne.CanvasObject {

			lbl := widget.NewLabel("")

			lbl.Wrapping = fyne.TextWrapOff

			return lbl

		},

		func(i widget.TableCellID, o fyne.CanvasObject) {

			lbl := o.(*widget.Label)

			if i.Row == 0 {

				// 表头行

				lbl.TextStyle = fyne.TextStyle{Bold: true}

				lbl.SetText(headers[i.Col])

			} else {

				// 数据行

				readers := st.GetAll()

				if i.Row-1 < len(readers) {

					row := readers[i.Row-1].ToRow()

					if i.Col < len(row) {

						lbl.TextStyle = fyne.TextStyle{}

						// 挂失状态红色，证件到期蓝色

						if readers[i.Row-1].LossStatus == "挂失" {

							lbl.TextStyle = fyne.TextStyle{Bold: true}

						}

						lbl.SetText(row[i.Col])

					}

				}

			}

		},

	)

	for col := 0; col < len(headers); col++ {

		tbl.SetColumnWidth(col, 80)

	}

	tbl.SetColumnWidth(10, 140) // 身份证号

	tbl.SetColumnWidth(13, 120) // 工作单位

	state.table = tbl



	// ============== 底部状态栏 ==============

	state.statusLabel = widget.NewLabel(fmt.Sprintf("合计: %d 条记录", st.Count()))

	statusBar := container.NewHBox(

		state.statusLabel,

		layout.NewSpacer(),

		widget.NewLabel(time.Now().Format("2006-01-02 15:04:05")),

	)



	// ============== 主布局 ==============

	content := container.NewBorder(

		container.NewVBox(tools, noticeBox, queryBox, widget.NewSeparator()),

		nil, nil, nil,

		container.NewScroll(tbl),

	)



	w.SetContent(container.NewBorder(nil, statusBar, nil, nil, content))

}



// refreshStatus 刷新表格和状态栏

func (s *mainWindowState) refreshStatus() {

	s.statusLabel.SetText(fmt.Sprintf("合计: %d 条记录", s.store.Count()))

	s.table.Refresh()

}



// ====================== 备份 / 恢复 / 清除 ======================



// backupData 备份数据到指定文件

func backupData(w fyne.Window, st *store.Store) {

	d := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {

		if err != nil || uc == nil {

			return

		}

		path := uc.URI().Path()

		uc.Close()



		if err := st.Backup(path); err != nil {

			dialog.ShowError(fmt.Errorf("备份失败: %w", err), w)

		} else {

			dialog.ShowInformation("备份成功",

				fmt.Sprintf("数据已成功备份到:\n%s\n共 %d 条记录", path, st.Count()), w)

		}

	}, w)



	d.SetFileName("读者信息备份_" + time.Now().Format("20060102_150405") + ".json")

	filter := storage.NewExtensionFileFilter([]string{".json"})

	d.SetFilter(filter)

	d.Show()

}



// restoreData 从备份文件恢复数据

func restoreData(w fyne.Window, st *store.Store, state *mainWindowState) {

	// 先确认

	dialog.ShowConfirm("确认恢复", "恢复数据将覆盖当前所有数据，是否继续？", func(ok bool) {

		if !ok {

			return

		}

		d := dialog.NewFileOpen(func(uc fyne.URIReadCloser, err error) {

			if err != nil || uc == nil {

				return

			}

			path := uc.URI().Path()

			uc.Close()



			if err := st.Restore(path); err != nil {

				dialog.ShowError(fmt.Errorf("恢复失败: %w", err), w)

			} else {

				state.refreshStatus()

				dialog.ShowInformation("恢复成功",

					fmt.Sprintf("数据已从备份文件恢复:\n%s", path), w)

			}

		}, w)



		filter := storage.NewExtensionFileFilter([]string{".json"})

		d.SetFilter(filter)

		d.Show()

	}, w)

}



// clearData 清除所有数据（自动备份后清除）

func clearData(w fyne.Window, st *store.Store, state *mainWindowState) {

	if st.Count() == 0 {

		dialog.ShowInformation("提示", "当前没有数据需要清除", w)

		return

	}



	dialog.ShowConfirm("确认清除",

		fmt.Sprintf("即将清除全部 %d 条读者记录\n系统将在清除前自动备份数据", st.Count()),

		func(ok bool) {

			if !ok {

				return

			}



			// 自动备份路径

			backupPath := fmt.Sprintf("读者信息_自动备份_%s.json",

				time.Now().Format("20060102_150405"))



			if err := st.Clear(backupPath); err != nil {

				dialog.ShowError(fmt.Errorf("清除失败: %w", err), w)

			} else {

				state.refreshStatus()

				dialog.ShowInformation("清除成功",

					fmt.Sprintf("数据已清除，备份保存在:\n%s", backupPath), w)

			}

		}, w)

}



// exportData 导出数据

func exportData(w fyne.Window, st *store.Store) {

	d := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {

		if err != nil || uc == nil {

			return

		}

		path := uc.URI().Path()

		uc.Close()



		if err := st.ExportCSV(path); err != nil {

			dialog.ShowError(fmt.Errorf("导出失败: %w", err), w)

		} else {

			dialog.ShowInformation("导出成功",

				fmt.Sprintf("数据已导出到:\n%s", path), w)

		}

	}, w)



	d.SetFileName("读者信息导出_" + time.Now().Format("20060102_150405") + ".csv")

	filter := storage.NewExtensionFileFilter([]string{".csv"})

	d.SetFilter(filter)

	d.Show()

}


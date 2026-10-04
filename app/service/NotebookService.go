package service

import (
	"context"
	"encoding/json"
	"errors"
	//	"fmt"
	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"sort"
	"strings"
	"time"
	//	"html"
)

// 笔记本

type NotebookService struct {
}

type notebookSortRequest struct {
	OperationID string         `json:"operationId,omitempty"`
	Sequences   map[string]int `json:"sequences"`
}

type notebookDragRequest struct {
	OperationID string   `json:"operationId,omitempty"`
	Current     string   `json:"current"`
	Parent      string   `json:"parent"`
	Siblings    []string `json:"siblings"`
}

// notebookOperationIdentity keeps a caller-provided generation as the receipt
// key while binding that generation into the immutable request digest. Legacy
// callers retain their request-derived identity and do not gain retry claims.
func notebookOperationIdentity(kind string, ownerID, resourceID domain.ObjectID, clientOperationID string, request any) (string, string, []byte, error) {
	requestIdentity, digest, desired, err := applicationnotes.NewOperationIdentity(kind, ownerID, resourceID, request)
	if err != nil || strings.TrimSpace(clientOperationID) == "" {
		return requestIdentity, digest, desired, err
	}
	operationID, err := applicationnotes.NewClientOperationIdentity(kind, ownerID, clientOperationID)
	if err != nil {
		return "", "", nil, err
	}
	return operationID, digest, desired, nil
}

// 排序
func sortSubNotebooks(eachNotebooks info.SubNotebooks) info.SubNotebooks {
	// 遍历子, 则子往上进行排序
	for _, eachNotebook := range eachNotebooks {
		if eachNotebook.Subs != nil && len(eachNotebook.Subs) > 0 {
			eachNotebook.Subs = sortSubNotebooks(eachNotebook.Subs)
		}
	}

	// 子排完了, 本层排
	sort.Sort(&eachNotebooks)
	return eachNotebooks
}

// 整理(成有关系)并排序
// GetNotebooks()调用
// ShareService调用
func ParseAndSortNotebooks(userNotebooks []info.Notebook, noParentDelete, needSort bool) info.SubNotebooks {
	// 整理成info.Notebooks
	// 第一遍, 建map
	// notebookId => info.Notebooks
	userNotebooksMap := make(map[ObjectID]*info.Notebooks, len(userNotebooks))
	for _, each := range userNotebooks {
		newNotebooks := info.Notebooks{Subs: info.SubNotebooks{}}
		newNotebooks.NotebookId = each.NotebookId
		newNotebooks.Title = each.Title
		//		newNotebooks.Title = html.EscapeString(each.Title)
		newNotebooks.Title = strings.Replace(strings.Replace(each.Title, "<script>", "", -1), "</script", "", -1)
		newNotebooks.Seq = each.Seq
		newNotebooks.UserId = each.UserId
		newNotebooks.ParentNotebookId = each.ParentNotebookId
		newNotebooks.NumberNotes = each.NumberNotes
		newNotebooks.IsTrash = each.IsTrash
		newNotebooks.IsBlog = each.IsBlog

		// 存地址
		userNotebooksMap[each.NotebookId] = &newNotebooks
	}

	// 第二遍, 追加到父下

	// 需要删除的id
	needDeleteNotebookId := map[ObjectID]bool{}
	for id, each := range userNotebooksMap {
		// 如果有父, 那么追加到父下, 并剪掉当前, 那么最后就只有根的元素
		if each.ParentNotebookId.Hex() != "" {
			if userNotebooksMap[each.ParentNotebookId] != nil {
				userNotebooksMap[each.ParentNotebookId].Subs = append(userNotebooksMap[each.ParentNotebookId].Subs, each) // Subs是存地址
				// 并剪掉
				// bug
				needDeleteNotebookId[id] = true
				// delete(userNotebooksMap, id)
			} else if noParentDelete {
				// 没有父, 且设置了要删除
				needDeleteNotebookId[id] = true
				// delete(userNotebooksMap, id)
			}
		}
	}

	// 第三遍, 得到所有根
	final := make(info.SubNotebooks, len(userNotebooksMap)-len(needDeleteNotebookId))
	i := 0
	for id, each := range userNotebooksMap {
		if !needDeleteNotebookId[id] {
			final[i] = each
			i++
		}
	}

	// 最后排序
	if needSort {
		return sortSubNotebooks(final)
	}
	return final
}

// 得到某notebook
func (this *NotebookService) GetNotebook(notebookId, userId string) info.Notebook {
	notebook := info.Notebook{}
	db.GetByIdAndUserId(db.Notebooks, notebookId, userId, &notebook)
	return notebook
}

func findNotebookForMutation(ctx context.Context, notebookId, userId string) (info.Notebook, error) {
	if db.Notebooks == nil {
		return info.Notebook{}, db.ErrMongoClientNotInitialized
	}
	var notebook info.Notebook
	err := db.Notebooks.FindContext(ctx, db.GetIdAndUserIdQ(notebookId, userId)).One(&notebook)
	return notebook, err
}
func (this *NotebookService) GetNotebookById(notebookId string) info.Notebook {
	notebook := info.Notebook{}
	db.Get(db.Notebooks, notebookId, &notebook)
	return notebook
}
func (this *NotebookService) GetNotebookByUserIdAndUrlTitle(userId, notebookIdOrUrlTitle string) info.Notebook {
	notebook := info.Notebook{}
	if IsObjectId(notebookIdOrUrlTitle) {
		db.Get(db.Notebooks, notebookIdOrUrlTitle, &notebook)
	} else {
		db.GetByQ(db.Notebooks, bson.M{"UserId": db.MustObjectIDFromHex(userId), "UrlTitle": encodeValue(notebookIdOrUrlTitle)}, &notebook)
	}
	return notebook
}

// 同步的方法
func (this *NotebookService) GeSyncNotebooks(userId string, afterUsn, maxEntry int) []info.Notebook {
	notebooks := []info.Notebook{}
	q := db.Notebooks.Find(bson.M{"UserId": db.MustObjectIDFromHex(userId), "Usn": bson.M{"$gt": afterUsn}})
	q.Sort("Usn").Limit(maxEntry).All(&notebooks)
	return notebooks
}

// GetActiveNotebooks returns the current notebook projection.  Unlike the
// sync query above, this view must not expose tombstones to normal list
// consumers after notebook deletion became logical.
func (this *NotebookService) GetActiveNotebooks(userId string) []info.Notebook {
	notebooks := []info.Notebook{}
	db.Notebooks.Find(activeNotebookQuery(userId)).Sort("Usn", "_id").All(&notebooks)
	return notebooks
}

func activeNotebookQuery(userId string) bson.M {
	return bson.M{"UserId": db.MustObjectIDFromHex(userId), "IsDeleted": bson.M{"$ne": true}}
}

// 得到用户下所有的notebook
// 排序好之后返回
// [ok]
func (this *NotebookService) GetNotebooks(userId string) info.SubNotebooks {
	userNotebooks := []info.Notebook{}
	orQ := []bson.M{
		bson.M{"IsDeleted": false},
		bson.M{"IsDeleted": bson.M{"$exists": false}},
	}
	db.Notebooks.Find(bson.M{"UserId": db.MustObjectIDFromHex(userId), "$or": orQ}).All(&userNotebooks)

	if len(userNotebooks) == 0 {
		return nil
	}

	return ParseAndSortNotebooks(userNotebooks, true, true)
}

// share调用, 不需要删除没有父的notebook
// 不需要排序, 因为会重新排序
// 通过notebookIds得到notebooks, 并转成层次有序
func (this *NotebookService) GetNotebooksByNotebookIds(notebookIds []ObjectID) info.SubNotebooks {
	userNotebooks := []info.Notebook{}
	db.Notebooks.Find(bson.M{"_id": bson.M{"$in": notebookIds}}).All(&userNotebooks)

	if len(userNotebooks) == 0 {
		return nil
	}

	return ParseAndSortNotebooks(userNotebooks, false, false)
}

// 添加
func (this *NotebookService) AddNotebook(notebook info.Notebook) (bool, info.Notebook) {

	if notebook.NotebookId.IsZero() {
		notebook.NotebookId = db.NewObjectID()
	}
	if notebook.UserId.IsZero() {
		return false, notebook
	}
	if !notebook.ParentNotebookId.IsZero() {
		parent := this.GetNotebook(notebook.ParentNotebookId.Hex(), notebook.UserId.Hex())
		if parent.NotebookId.IsZero() || parent.IsDeleted || parent.NotebookId == notebook.NotebookId {
			return false, notebook
		}
	}

	notebook.UrlTitle = GetUrTitle(notebook.UserId.Hex(), notebook.Title, "notebook", notebook.NotebookId.Hex())
	usn, err := userService.AllocateUsn(context.Background(), notebook.UserId.Hex())
	if err != nil {
		return false, notebook
	}
	notebook.Usn = usn
	now := time.Now()
	notebook.CreatedTime = now
	notebook.UpdatedTime = now
	err = db.Notebooks.Insert(notebook)
	if err != nil {
		return false, notebook
	}
	return true, notebook
}

// 更新笔记, api
func (this *NotebookService) UpdateNotebookApi(userId, notebookId, title, parentNotebookId string, seq, usn int) (bool, string, info.Notebook) {
	if notebookId == "" {
		return false, "notebookIdNotExists", info.Notebook{}
	}

	// 先判断usn是否和数据库的一样, 如果不一样, 则冲突, 不保存
	notebook, lookupErr := findNotebookForMutation(context.Background(), notebookId, userId)
	if lookupErr != nil && !errors.Is(lookupErr, mongo.ErrNoDocuments) {
		return false, "storage", info.Notebook{}
	}
	// 不存在
	if notebook.NotebookId.IsZero() || notebook.IsDeleted {
		return false, "notExists", notebook
	} else if notebook.Usn != usn {
		return false, "conflict", notebook
	}
	notebook.Title = title

	updates := bson.M{"Title": title, "Seq": seq, "UpdatedTime": time.Now()}
	if parentNotebookId != "" {
		if !db.IsValidObjectIDHex(parentNotebookId) {
			return false, "validation", notebook
		}
		if parentNotebookId == notebookId {
			return false, "validation", notebook
		}
		parent, parentErr := findNotebookForMutation(context.Background(), parentNotebookId, userId)
		if parentErr != nil && !errors.Is(parentErr, mongo.ErrNoDocuments) {
			return false, "storage", notebook
		}
		descendant, descendantErr := this.isNotebookDescendant(userId, parentNotebookId, notebookId)
		if descendantErr != nil {
			return false, "storage", notebook
		}
		if parent.NotebookId.IsZero() || parent.IsDeleted || descendant {
			return false, "validation", notebook
		}
		updates["ParentNotebookId"] = db.MustObjectIDFromHex(parentNotebookId)
	} else {
		updates["ParentNotebookId"] = ""
	}
	afterUSN, err := userService.AllocateUsn(context.Background(), userId)
	if err != nil {
		return false, "storage", notebook
	}
	notebook.Usn = afterUSN
	updates["Usn"] = afterUSN
	err = db.Notebooks.UpdateOneMatchedContext(context.Background(), bson.M{
		"_id":       db.MustObjectIDFromHex(notebookId),
		"UserId":    db.MustObjectIDFromHex(userId),
		"Usn":       usn,
		"IsDeleted": bson.M{"$ne": true},
	}, bson.M{"$set": updates})
	if err == nil {
		return true, "", this.GetNotebook(notebookId, userId)
	}
	if errors.Is(err, db.ErrDocumentNotFound) {
		return false, "conflict", notebook
	}
	return false, "storage", notebook
}

// 判断是否是blog
func (this *NotebookService) IsBlog(notebookId string) bool {
	notebook := info.Notebook{}
	db.GetByQWithFields(db.Notebooks, bson.M{"_id": db.MustObjectIDFromHex(notebookId)}, []string{"IsBlog"}, &notebook)
	return notebook.IsBlog
}

// 判断是否是我的notebook
func (this *NotebookService) IsMyNotebook(notebookId, userId string) bool {
	if !db.IsValidObjectIDHex(notebookId) || !db.IsValidObjectIDHex(userId) {
		return false
	}
	return db.Has(db.Notebooks, bson.M{
		"_id":       db.MustObjectIDFromHex(notebookId),
		"UserId":    db.MustObjectIDFromHex(userId),
		"IsDeleted": bson.M{"$ne": true},
	})
}

// 更新笔记本信息
// 太广, 不用
/*
func (this *NotebookService) UpdateNotebook(notebook info.Notebook) bool {
	return db.UpdateByIdAndUserId2(db.Notebooks, notebook.NotebookId, notebook.UserId, notebook)
}
*/

// 更新笔记本标题
// [ok]
func (this *NotebookService) UpdateNotebookTitle(notebookId, userId, title string) bool {
	if !db.IsValidObjectIDHex(notebookId) || !db.IsValidObjectIDHex(userId) {
		return false
	}
	notebook := this.GetNotebook(notebookId, userId)
	if notebook.NotebookId.IsZero() || notebook.IsDeleted {
		return false
	}
	usn, err := userService.AllocateUsn(context.Background(), userId)
	if err != nil {
		return false
	}
	return db.Notebooks.UpdateOneMatchedContext(context.Background(), bson.M{"_id": notebook.NotebookId, "UserId": notebook.UserId, "Usn": notebook.Usn, "IsDeleted": false}, bson.M{"$set": bson.M{"Title": title, "Usn": usn}}) == nil
}

// ToBlog or Not
func (this *NotebookService) ToBlog(userId, notebookId string, isBlog bool) bool {
	return this.toBlogWithReceipt(userId, notebookId, isBlog)
}

// 查看是否有子notebook
// 先查看该notebookId下是否有notes, 没有则删除
func (this *NotebookService) DeleteNotebook(userId, notebookId string) (bool, string) {
	if !db.IsValidObjectIDHex(userId) || !db.IsValidObjectIDHex(notebookId) {
		return false, "notExists"
	}
	notebook, lookupErr := findNotebookForMutation(context.Background(), notebookId, userId)
	if lookupErr != nil && !errors.Is(lookupErr, mongo.ErrNoDocuments) {
		return false, "storage"
	}
	if notebook.NotebookId.IsZero() || notebook.IsDeleted {
		return false, "notExists"
	}
	childCount, err := db.Notebooks.FindContext(context.Background(), bson.M{
		"ParentNotebookId": db.MustObjectIDFromHex(notebookId),
		"UserId":           db.MustObjectIDFromHex(userId),
		"IsDeleted":        false,
	}).Count()
	if err != nil {
		return false, "storage"
	}
	if childCount == 0 { // 无
		noteCount, countErr := db.Notes.FindContext(context.Background(), bson.M{"NotebookId": db.MustObjectIDFromHex(notebookId),
			"UserId":    db.MustObjectIDFromHex(userId),
			"IsTrash":   false,
			"IsDeleted": false}).Count()
		if countErr != nil {
			return false, "storage"
		}
		if noteCount == 0 { // 不包含trash
			// 不是真删除 1/20, 为了同步笔记本
			usn, err := userService.AllocateUsn(context.Background(), userId)
			if err != nil {
				return false, "storage"
			}
			err = db.Notebooks.UpdateOneMatchedContext(context.Background(), bson.M{
				"_id":       notebook.NotebookId,
				"UserId":    notebook.UserId,
				"Usn":       notebook.Usn,
				"IsDeleted": false,
			}, bson.M{"$set": bson.M{"IsDeleted": true, "Usn": usn, "UpdatedTime": time.Now()}})
			if err != nil {
				if errors.Is(err, db.ErrDocumentNotFound) {
					return false, "conflict"
				}
				return false, "storage"
			}
			return true, ""
			//			return db.DeleteByIdAndUserId(db.Notebooks, notebookId, userId), ""
		}
		return false, "笔记本下有笔记"
	} else {
		return false, "笔记本下有子笔记本"
	}
}

// API调用, 删除笔记本, 不作笔记控制
func (this *NotebookService) DeleteNotebookForce(userId, notebookId string, usn int) (bool, string) {
	if !db.IsValidObjectIDHex(userId) || !db.IsValidObjectIDHex(notebookId) {
		return false, "notExists"
	}
	notebook, lookupErr := findNotebookForMutation(context.Background(), notebookId, userId)
	if lookupErr != nil && !errors.Is(lookupErr, mongo.ErrNoDocuments) {
		return false, "storage"
	}
	// 不存在
	if notebook.NotebookId.IsZero() || notebook.IsDeleted {
		return false, "notExists"
	} else if notebook.Usn != usn {
		return false, "conflict"
	}
	afterUsn, err := userService.AllocateUsn(context.Background(), userId)
	if err != nil {
		return false, "storage"
	}
	err = db.Notebooks.UpdateOneMatchedContext(context.Background(), bson.M{
		"_id":       db.MustObjectIDFromHex(notebookId),
		"UserId":    db.MustObjectIDFromHex(userId),
		"Usn":       usn,
		"IsDeleted": false,
	}, bson.M{"$set": bson.M{"IsDeleted": true, "Usn": afterUsn, "UpdatedTime": time.Now()}})
	if err != nil {
		if errors.Is(err, db.ErrDocumentNotFound) {
			return false, "conflict"
		}
		return false, "storage"
	}
	return true, ""
}

// 排序
// 传入 notebookId => Seq
// 为什么要传入userId, 防止修改其它用户的信息 (恶意)
// [ok]
func (this *NotebookService) SortNotebooks(userId string, notebookId2Seqs map[string]int, clientOperationIDs ...string) bool {
	if len(notebookId2Seqs) == 0 || !db.IsValidObjectIDHex(userId) {
		return false
	}
	clientOperationID := firstClientOperationID(clientOperationIDs)
	ownerID := db.MustObjectIDFromHex(userId)
	ids := make([]string, 0, len(notebookId2Seqs))
	for notebookID := range notebookId2Seqs {
		if !db.IsValidObjectIDHex(notebookID) {
			return false
		}
		ids = append(ids, notebookID)
	}
	sort.Strings(ids)
	resourceID := db.MustObjectIDFromHex(ids[0])
	request := notebookSortRequest{OperationID: clientOperationID, Sequences: notebookId2Seqs}
	operationID, digest, desired, err := notebookOperationIdentity("notebook_sort", ownerID, resourceID, clientOperationID, request)
	if err != nil {
		return false
	}
	before := make(map[string]info.Notebook, len(notebookId2Seqs))
	receipt, getErr := db.GetWorkspaceOperation(context.Background(), ownerID, operationID)
	switch {
	case getErr == nil:
		if receipt.Status == applicationnotes.OperationCommitted {
			current := make(map[string]info.Notebook, len(ids))
			unchanged := true
			for _, notebookID := range ids {
				notebook := this.GetNotebook(notebookID, userId)
				if notebook.NotebookId.IsZero() || notebook.IsDeleted || notebook.Seq != notebookId2Seqs[notebookID] || notebook.Usn != receipt.StepUSNs["notebook:"+notebookID] {
					unchanged = false
				}
				current[notebookID] = notebook
			}
			if unchanged {
				return true
			}
			if strings.TrimSpace(clientOperationID) != "" {
				return false
			}
			before = current
			generations := make(map[string]int, len(current))
			for id, notebook := range current {
				generations[id] = notebook.Usn
			}
			operationID, digest, desired, err = applicationnotes.NewOperationIdentity("notebook_sort", ownerID, resourceID, struct {
				Sequences   map[string]int
				Generations map[string]int
			}{Sequences: notebookId2Seqs, Generations: generations})
			if err != nil {
				return false
			}
			getErr = mongo.ErrNoDocuments
		}
		if len(receipt.BeforeState) == 0 {
			return false
		}
		var frozen map[string]info.Notebook
		if err := json.Unmarshal(receipt.BeforeState, &frozen); err != nil || !sameNotebookIDSet(frozen, ids) {
			return false
		}
		before = frozen
	case errors.Is(getErr, mongo.ErrNoDocuments):
		for _, notebookID := range ids {
			notebook := this.GetNotebook(notebookID, userId)
			if notebook.NotebookId.IsZero() || notebook.IsDeleted {
				return false
			}
			before[notebookID] = notebook
		}
		generations := make(map[string]int, len(before))
		for id, notebook := range before {
			generations[id] = notebook.Usn
		}
		if strings.TrimSpace(clientOperationID) == "" {
			operationID, digest, desired, err = applicationnotes.NewOperationIdentity("notebook_sort", ownerID, resourceID, struct {
				Sequences   map[string]int
				Generations map[string]int
			}{Sequences: notebookId2Seqs, Generations: generations})
			if err != nil {
				return false
			}
		}
	default:
		// A failed receipt lookup is not evidence that the operation is new.
		// Rebuilding a baseline here could create a second identity and replay
		// an old reorder over a concurrent mutation.
		return false
	}
	allDesired := true
	for _, id := range ids {
		if before[id].Seq != notebookId2Seqs[id] {
			allDesired = false
			break
		}
	}
	if allDesired && getErr != nil {
		return true
	}
	beforeState, _ := json.Marshal(before)
	plan := db.WorkspaceMutationPlan{OperationID: operationID, OwnerID: ownerID, ResourceID: resourceID, Kind: "notebook_sort", InputDigest: digest, BeforeState: beforeState, DesiredState: desired, FailurePolicy: applicationnotes.FailurePending}
	plan.RestoreBeforeState = func(payload []byte) error {
		var frozen map[string]info.Notebook
		if err := json.Unmarshal(payload, &frozen); err != nil {
			return err
		}
		if !sameNotebookIDSet(frozen, ids) {
			return errors.New("notebook sort before state target set changed")
		}
		before = frozen
		return nil
	}
	for _, id := range ids {
		notebookID := id
		seq := notebookId2Seqs[id]
		assignedUSN := 0
		plan.Steps = append(plan.Steps, db.WorkspaceMutationStep{
			Name: "notebook:" + notebookID, ReplaySafe: true,
			AssignedUSN:        func() int { return assignedUSN },
			RestoreAssignedUSN: func(usn int) { assignedUSN = usn },
			Apply: func(ctx context.Context) error {
				original := before[notebookID]
				usn := assignedUSN
				if usn <= 0 {
					var err error
					usn, err = userService.AllocateUsn(ctx, userId)
					if err != nil {
						return err
					}
					assignedUSN = usn
				}
				return db.Notebooks.UpdateOneMatchedContext(ctx, bson.M{"_id": original.NotebookId, "UserId": ownerID, "Usn": original.Usn, "IsDeleted": bson.M{"$ne": true}}, bson.M{"$set": bson.M{"Seq": seq, "Usn": usn}})
			},
			Verify: func(ctx context.Context) (bool, error) {
				original := before[notebookID]
				if assignedUSN <= 0 {
					return false, nil
				}
				var found info.Notebook
				err := db.Notebooks.FindContext(ctx, bson.M{"_id": original.NotebookId, "UserId": ownerID, "Seq": seq, "Usn": assignedUSN, "IsDeleted": bson.M{"$ne": true}}).One(&found)
				if errors.Is(err, mongo.ErrNoDocuments) {
					return false, nil
				}
				return err == nil, err
			},
		})
	}
	result, err := db.RunWorkspaceRepair(context.Background(), plan)
	return err == nil && result.Committed
}

// 排序和设置父
func (this *NotebookService) DragNotebooks(userId string, curNotebookId string, parentNotebookId string, siblings []string, clientOperationIDs ...string) bool {
	if !db.IsValidObjectIDHex(userId) || !db.IsValidObjectIDHex(curNotebookId) {
		return false
	}
	clientOperationID := firstClientOperationID(clientOperationIDs)
	ownerID := db.MustObjectIDFromHex(userId)
	resourceID := db.MustObjectIDFromHex(curNotebookId)
	if parentNotebookId != "" && (!db.IsValidObjectIDHex(parentNotebookId) || parentNotebookId == curNotebookId) {
		return false
	}

	seen := map[string]bool{}
	before := notebookDragBeforeState{Targets: make(map[string]info.Notebook, len(siblings)+1)}
	updates := make(map[string]bson.M, len(siblings)+1)
	parentValue := interface{}("")
	if parentNotebookId != "" {
		parentValue = db.MustObjectIDFromHex(parentNotebookId)
	}
	updates[curNotebookId] = bson.M{"ParentNotebookId": parentValue}
	for _, notebookId := range siblings {
		if !db.IsValidObjectIDHex(notebookId) || seen[notebookId] {
			return false
		}
		seen[notebookId] = true
		if updates[notebookId] == nil {
			updates[notebookId] = bson.M{}
		}
		updates[notebookId]["Seq"] = len(seen) - 1
	}
	ids := make([]string, 0, len(updates))
	for id := range updates {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	request := notebookDragRequest{OperationID: clientOperationID, Current: curNotebookId, Parent: parentNotebookId, Siblings: siblings}
	operationID, digest, desired, err := notebookOperationIdentity("notebook_drag", ownerID, resourceID, clientOperationID, request)
	if err != nil {
		return false
	}
	receipt, getErr := db.GetWorkspaceOperation(context.Background(), ownerID, operationID)
	switch {
	case getErr == nil:
		if receipt.Status == applicationnotes.OperationCommitted {
			current := notebookDragBeforeState{Targets: make(map[string]info.Notebook, len(ids))}
			unchanged := true
			for _, notebookID := range ids {
				notebook := this.GetNotebook(notebookID, userId)
				current.Targets[notebookID] = notebook
				if notebook.NotebookId.IsZero() || notebook.IsDeleted || notebook.Usn != receipt.StepUSNs["notebook:"+notebookID] || !notebookMatchesDragFields(notebook, updates[notebookID]) {
					unchanged = false
				}
			}
			if parentNotebookId != "" {
				parent := this.GetNotebook(parentNotebookId, userId)
				current.Parent = &parent
				if parent.NotebookId.IsZero() || parent.IsDeleted || parent.Usn != receipt.StepUSNs["parent"] {
					unchanged = false
				}
			}
			if unchanged {
				return true
			}
			if strings.TrimSpace(clientOperationID) != "" {
				return false
			}
			before = current
			operationID, digest, desired, err = notebookDragGenerationIdentity(ownerID, resourceID, request, before)
			if err != nil {
				return false
			}
			getErr = mongo.ErrNoDocuments
		}
		if len(receipt.BeforeState) == 0 {
			return false
		}
		var frozen notebookDragBeforeState
		if err := json.Unmarshal(receipt.BeforeState, &frozen); err != nil || !sameNotebookIDSet(frozen.Targets, ids) || !frozenParentMatchesRequest(frozen.Parent, parentNotebookId) {
			return false
		}
		before = frozen
	case errors.Is(getErr, mongo.ErrNoDocuments):
		current := this.GetNotebook(curNotebookId, userId)
		if current.NotebookId.IsZero() || current.IsDeleted {
			return false
		}
		before.Targets[curNotebookId] = current
		if parentNotebookId != "" {
			parent, parentErr := findNotebookForMutation(context.Background(), parentNotebookId, userId)
			if parentErr != nil {
				return false
			}
			descendant, descendantErr := this.isNotebookDescendant(userId, parentNotebookId, curNotebookId)
			if descendantErr != nil || parent.NotebookId.IsZero() || parent.IsDeleted || descendant {
				return false
			}
			before.Parent = &parent
		}
		for _, notebookId := range siblings {
			notebook := this.GetNotebook(notebookId, userId)
			if notebook.NotebookId.IsZero() || notebook.IsDeleted {
				return false
			}
			before.Targets[notebookId] = notebook
		}
		if strings.TrimSpace(clientOperationID) == "" {
			operationID, digest, desired, err = notebookDragGenerationIdentity(ownerID, resourceID, request, before)
			if err != nil {
				return false
			}
		}
	default:
		// A receipt lookup error is not evidence that the operation is new.
		return false
	}
	plan := db.WorkspaceMutationPlan{
		OperationID: operationID, OwnerID: ownerID, ResourceID: resourceID,
		Kind: "notebook_drag", InputDigest: digest, DesiredState: desired,
		FailurePolicy: applicationnotes.FailurePending,
	}
	plan.BeforeState, _ = json.Marshal(before)
	plan.RestoreBeforeState = func(payload []byte) error {
		var frozen notebookDragBeforeState
		if err := json.Unmarshal(payload, &frozen); err != nil {
			return err
		}
		if !sameNotebookIDSet(frozen.Targets, ids) || !frozenParentMatchesRequest(frozen.Parent, parentNotebookId) {
			return errors.New("notebook drag before state target set changed")
		}
		before = frozen
		return nil
	}
	if before.Parent != nil {
		plan.Steps = append(plan.Steps, db.WorkspaceMutationStep{
			Name:       "parent",
			ReplaySafe: true,
			AssignedUSN: func() int {
				return before.Parent.Usn
			},
			Apply: func(ctx context.Context) error {
				return verifyFrozenDragParent(ctx, before.Parent, ownerID)
			},
			Verify: func(ctx context.Context) (bool, error) {
				if err := verifyFrozenDragParent(ctx, before.Parent, ownerID); err != nil {
					return false, err
				}
				return true, nil
			},
		})
	}
	for _, id := range ids {
		notebookID := id
		assignedUSN := 0
		fields := bson.M{}
		for key, value := range updates[id] {
			fields[key] = value
		}
		plan.Steps = append(plan.Steps, db.WorkspaceMutationStep{
			Name: "notebook:" + notebookID, ReplaySafe: true,
			AssignedUSN:        func() int { return assignedUSN },
			RestoreAssignedUSN: func(usn int) { assignedUSN = usn },
			Apply: func(ctx context.Context) error {
				if err := verifyFrozenDragParent(ctx, before.Parent, ownerID); err != nil {
					return err
				}
				original := before.Targets[notebookID]
				usn := assignedUSN
				if usn <= 0 {
					var err error
					usn, err = userService.AllocateUsn(ctx, userId)
					if err != nil {
						return err
					}
					assignedUSN = usn
				}
				patch := bson.M{}
				for key, value := range fields {
					patch[key] = value
				}
				patch["Usn"] = usn
				patch["UpdatedTime"] = time.Now()
				return db.Notebooks.UpdateOneMatchedContext(ctx, bson.M{"_id": original.NotebookId, "UserId": original.UserId, "Usn": original.Usn, "IsDeleted": bson.M{"$ne": true}}, bson.M{"$set": patch})
			},
			Verify: func(ctx context.Context) (bool, error) {
				if err := verifyFrozenDragParent(ctx, before.Parent, ownerID); err != nil {
					return false, err
				}
				original := before.Targets[notebookID]
				if assignedUSN <= 0 {
					return false, nil
				}
				filter := bson.M{"_id": original.NotebookId, "UserId": original.UserId, "IsDeleted": bson.M{"$ne": true}}
				for key, value := range fields {
					filter[key] = value
				}
				filter["Usn"] = assignedUSN
				var found info.Notebook
				err := db.Notebooks.FindContext(ctx, filter).One(&found)
				if errors.Is(err, mongo.ErrNoDocuments) {
					return false, nil
				}
				return err == nil, err
			},
		})
	}
	result, err := db.RunWorkspaceRepair(context.Background(), plan)
	return err == nil && result.Committed
}

func notebookDragGenerationIdentity(ownerID, resourceID domain.ObjectID, request any, before notebookDragBeforeState) (string, string, []byte, error) {
	generations := make(map[string]int, len(before.Targets)+1)
	for id, notebook := range before.Targets {
		generations[id] = notebook.Usn
	}
	if before.Parent != nil {
		generations["parent"] = before.Parent.Usn
	}
	return applicationnotes.NewOperationIdentity("notebook_drag", ownerID, resourceID, struct {
		Request     any
		Generations map[string]int
	}{Request: request, Generations: generations})
}

func firstClientOperationID(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func notebookMatchesDragFields(notebook info.Notebook, fields bson.M) bool {
	for key, value := range fields {
		switch key {
		case "Seq":
			seq, ok := value.(int)
			if !ok || notebook.Seq != seq {
				return false
			}
		case "ParentNotebookId":
			parent, ok := value.(ObjectID)
			if ok {
				if notebook.ParentNotebookId != parent {
					return false
				}
				continue
			}
			root, rootOK := value.(string)
			if !rootOK || root != "" || !notebook.ParentNotebookId.IsZero() {
				return false
			}
		}
	}
	return true
}

type notebookDragBeforeState struct {
	Targets map[string]info.Notebook
	Parent  *info.Notebook
}

func frozenParentMatchesRequest(parent *info.Notebook, parentNotebookID string) bool {
	if parentNotebookID == "" {
		return parent == nil
	}
	return parent != nil && parent.NotebookId.Hex() == parentNotebookID && !parent.IsDeleted
}

func verifyFrozenDragParent(ctx context.Context, parent *info.Notebook, ownerID ObjectID) error {
	if parent == nil {
		return nil
	}
	var current info.Notebook
	err := db.Notebooks.FindContext(ctx, bson.M{
		"_id": parent.NotebookId, "UserId": ownerID,
		"Usn": parent.Usn, "IsDeleted": bson.M{"$ne": true},
	}).One(&current)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return db.ErrDocumentNotFound
	}
	return err
}

func sameNotebookIDSet(notebooks map[string]info.Notebook, ids []string) bool {
	if len(notebooks) != len(ids) {
		return false
	}
	for _, id := range ids {
		if _, ok := notebooks[id]; !ok {
			return false
		}
	}
	return true
}

func (this *NotebookService) isNotebookDescendant(userId, candidateParentId, currentId string) (bool, error) {
	visited := map[string]bool{}
	for candidateParentId != "" {
		if candidateParentId == currentId || visited[candidateParentId] {
			return true, nil
		}
		visited[candidateParentId] = true
		parent, err := findNotebookForMutation(context.Background(), candidateParentId, userId)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return false, nil
			}
			return false, err
		}
		if parent.NotebookId.IsZero() || parent.IsDeleted {
			return false, nil
		}
		candidateParentId = parent.ParentNotebookId.Hex()
	}
	return false, nil
}

// 重新统计笔记本下的笔记数目
// noteSevice: AddNote, CopyNote, CopySharedNote, MoveNote
// trashService: DeleteNote (recove不用, 都统一在MoveNote里了)
func (this *NotebookService) ReCountNotebookNumberNotes(notebookId string) bool {
	if !db.IsValidObjectIDHex(notebookId) {
		return false
	}
	notebook := this.GetNotebookById(notebookId)
	if notebook.NotebookId.IsZero() || notebook.UserId.IsZero() {
		return false
	}
	return this.reCountNotebookNumberNotes(context.Background(), notebook.NotebookId, notebook.UserId) == nil
}

func (this *NotebookService) reCountNotebookNumberNotes(ctx context.Context, notebookID, ownerID ObjectID) error {
	var notebook info.Notebook
	err := db.Notebooks.FindContext(ctx, bson.M{"_id": notebookID, "UserId": ownerID}).One(&notebook)
	if errors.Is(err, mongo.ErrNoDocuments) || notebookCountProjectionSatisfied(notebook) {
		// A missing or tombstoned notebook has no live count projection to
		// maintain. This is the terminal state for cleanup after its notes are
		// permanently deleted.
		return nil
	}
	if err != nil {
		return err
	}
	count, err := db.Notes.FindContext(ctx, bson.M{
		"NotebookId": notebookID,
		"UserId":     ownerID,
		"IsTrash":    false,
		"IsDeleted":  false,
	}).Count()
	if err != nil {
		return err
	}
	return db.Notebooks.UpdateOneMatchedContext(ctx,
		bson.M{"_id": notebookID, "UserId": ownerID, "IsDeleted": bson.M{"$ne": true}},
		bson.M{"$set": bson.M{"NumberNotes": count}},
	)
}

func (this *NotebookService) verifyNotebookNumberNotes(ctx context.Context, notebookID, ownerID ObjectID) (bool, error) {
	var notebook info.Notebook
	err := db.Notebooks.FindContext(ctx, bson.M{"_id": notebookID, "UserId": ownerID}).One(&notebook)
	if errors.Is(err, mongo.ErrNoDocuments) || notebookCountProjectionSatisfied(notebook) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	count, err := db.Notes.FindContext(ctx, bson.M{
		"NotebookId": notebookID,
		"UserId":     ownerID,
		"IsTrash":    false,
		"IsDeleted":  false,
	}).Count()
	if err != nil {
		return false, err
	}
	err = db.Notebooks.FindContext(ctx, bson.M{
		"_id": notebookID, "UserId": ownerID, "IsDeleted": bson.M{"$ne": true}, "NumberNotes": count,
	}).One(&notebook)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	return err == nil, err
}

func notebookCountProjectionSatisfied(notebook info.Notebook) bool {
	return notebook.IsDeleted
}

func (this *NotebookService) ReCountAll() {
	/*
		// 得到所有笔记本
		notebooks := []info.Notebook{}
		db.ListByQWithFields(db.Notebooks, bson.M{}, []string{"NotebookId"}, &notebooks)

		for _, each := range notebooks {
			this.ReCountNotebookNumberNotes(each.NotebookId.Hex())
		}
	*/
}

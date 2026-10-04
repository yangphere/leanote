package service

import (
	"context"
	"errors"
	"time"

	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const IMAGE_TYPE = 0

type AlbumService struct {
}

// add album
func (this *AlbumService) AddAlbum(album info.Album) bool {
	created, err := this.CreateAlbum(context.Background(), album.UserId.Hex(), album.Name, album.AlbumId, album.Seq)
	return err == nil && !created.AlbumId.IsZero()
}

// get albums
func (this *AlbumService) GetAlbums(userId string) []info.Album {
	albums, err := this.ListAlbums(context.Background(), userId)
	if err != nil {
		return []info.Album{}
	}
	return albums
}

// delete album
// presupposition: has no images under this ablum
func (this *AlbumService) DeleteAlbum(userId, albumId string) (bool, string) {
	err := this.DeleteAlbumResult(context.Background(), userId, albumId)
	if err == nil {
		return true, ""
	}
	var contentErr *applicationcontent.Error
	if errors.As(err, &contentErr) && contentErr.Code == "album_has_images" {
		return false, "has images"
	}
	return false, ""
}

// update album name
func (this *AlbumService) UpdateAlbum(albumId, userId, name string) bool {
	return this.UpdateAlbumResult(context.Background(), userId, albumId, name) == nil
}

func (this *AlbumService) ListAlbums(ctx context.Context, actorIDText string) ([]info.Album, error) {
	actorID, err := strictImageObjectID(actorIDText, "album_actor")
	if err != nil {
		return nil, err
	}
	if db.Albums == nil {
		return nil, applicationcontent.NewError(applicationcontent.ErrorDependency, "album_store_unavailable", nil)
	}
	albums := []info.Album{}
	if err := db.Albums.FindContext(ctx, bson.M{"UserId": actorID}).All(&albums); err != nil {
		return nil, imageRepositoryError("album_list", err)
	}
	return albums, nil
}

func (this *AlbumService) CreateAlbum(ctx context.Context, actorIDText, name string, albumID domain.ObjectID, seq int) (info.Album, error) {
	actorID, err := strictImageObjectID(actorIDText, "album_actor")
	if err != nil {
		return info.Album{}, err
	}
	name, err = applicationcontent.CleanVisibleText(name, false)
	if err != nil {
		return info.Album{}, err
	}
	if albumID.IsZero() {
		albumID = db.NewObjectID()
	}
	album := info.Album{AlbumId: albumID, UserId: actorID, Name: name, Type: IMAGE_TYPE, Seq: seq, CreatedTime: time.Now()}
	if db.Albums == nil {
		return info.Album{}, applicationcontent.NewError(applicationcontent.ErrorDependency, "album_store_unavailable", nil)
	}
	if err := db.Albums.InsertContext(ctx, album); err != nil {
		return info.Album{}, imageRepositoryError("album_create", err)
	}
	return album, nil
}

func (this *AlbumService) DeleteAlbumResult(ctx context.Context, actorIDText, albumIDText string) error {
	actorID, err := strictImageObjectID(actorIDText, "album_actor")
	if err != nil {
		return err
	}
	mutation, err := applicationcontent.NormalizeAlbumMutation(albumIDText, "delete")
	if err != nil {
		return err
	}
	if db.Albums == nil || db.Files == nil {
		return applicationcontent.NewError(applicationcontent.ErrorDependency, "album_store_unavailable", nil)
	}
	var album info.Album
	if err := db.Albums.FindContext(ctx, bson.M{"_id": mutation.AlbumID, "UserId": actorID}).One(&album); err != nil {
		return imageRepositoryError("album_delete_lookup", err)
	}
	count, err := db.Files.FindContext(ctx, bson.M{"AlbumId": mutation.AlbumID, "UserId": actorID}).Count()
	if err != nil {
		return imageRepositoryError("album_image_count", err)
	}
	if count != 0 {
		return applicationcontent.NewError(applicationcontent.ErrorConflict, "album_has_images", nil)
	}
	if err := db.Albums.RemoveContext(ctx, bson.M{"_id": mutation.AlbumID, "UserId": actorID}); err != nil {
		return imageRepositoryError("album_delete", err)
	}
	return nil
}

func (this *AlbumService) UpdateAlbumResult(ctx context.Context, actorIDText, albumIDText, name string) error {
	actorID, err := strictImageObjectID(actorIDText, "album_actor")
	if err != nil {
		return err
	}
	mutation, err := applicationcontent.NormalizeAlbumMutation(albumIDText, name)
	if err != nil {
		return err
	}
	if db.Albums == nil {
		return applicationcontent.NewError(applicationcontent.ErrorDependency, "album_store_unavailable", nil)
	}
	if err := db.Albums.UpdateOneMatchedContext(ctx, bson.M{"_id": mutation.AlbumID, "UserId": actorID}, bson.M{"$set": bson.M{"Name": mutation.Name}}); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return applicationcontent.NewError(applicationcontent.ErrorNotFound, "album_update", err)
		}
		return imageRepositoryError("album_update", err)
	}
	return nil
}

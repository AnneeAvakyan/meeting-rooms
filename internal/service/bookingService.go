package service

import (
	"github.com/AnneeAvakyan/meeting-rooms/domain/models"
	"github.com/AnneeAvakyan/meeting-rooms/domain/repository"
)

type BookingService struct {
	bookingRepo repository.BookingRepository
	booking     *models.Booking
}

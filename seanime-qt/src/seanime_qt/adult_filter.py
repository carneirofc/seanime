"""AdultFilterProxy — split a media model into safe / adult halves.

A ``QSortFilterProxyModel`` that keeps only the rows whose ``isAdult`` role
matches a fixed target. Two instances over the same source model give the
"split adult content" behaviour (a safe grid plus a separate adult grid) that
the server's ``anilist.splitAdultContent`` setting asks for, with no extra
work on the source model.
"""

from __future__ import annotations

from PySide6.QtCore import QModelIndex, QSortFilterProxyModel

from .library_model import LibraryModel


class AdultFilterProxy(QSortFilterProxyModel):
    """Accepts only rows whose ``isAdult`` equals ``want_adult``.

    The ``isAdult`` role number is identical across LibraryModel and SearchModel
    (both derive it from ``QAbstractListModel``'s role enum the same way), so a
    single role constant works for either source.
    """

    _IS_ADULT_ROLE = LibraryModel.IsAdultRole

    def __init__(self, want_adult: bool, parent=None) -> None:
        super().__init__(parent)
        self._want_adult = bool(want_adult)

    def filterAcceptsRow(self, source_row: int, source_parent: QModelIndex) -> bool:
        model = self.sourceModel()
        if model is None:
            return False
        index = model.index(source_row, 0, source_parent)
        return bool(model.data(index, self._IS_ADULT_ROLE)) == self._want_adult

    @property
    def count(self) -> int:
        return self.rowCount()

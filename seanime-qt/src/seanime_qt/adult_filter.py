"""AdultFilterProxy — split a media model into safe / adult halves.

A ``QSortFilterProxyModel`` that keeps only the rows whose ``isAdult`` role
matches a fixed target. Two instances over the same source model give the
"split adult content" behaviour (a safe grid plus a separate adult grid) that
the server's ``anilist.splitAdultContent`` setting asks for, with no extra
work on the source model.
"""

from __future__ import annotations

from PySide6.QtCore import Property, QModelIndex, QSortFilterProxyModel, Signal

from .library_model import LibraryModel


class AdultFilterProxy(QSortFilterProxyModel):
    """Accepts only rows whose ``isAdult`` equals ``wantAdult``.

    The ``isAdult`` role number is identical across LibraryModel, SearchModel and
    MangaLibraryModel (all derive it from ``QAbstractListModel``'s role enum the
    same way), so a single role constant works for any of them.

    Constructible both from Python (``AdultFilterProxy(True, parent)``, as the
    library/search/manga splits do) and from QML: it is registered as a type so a
    ``MediaCarousel`` can spin up its own safe/adult pair with ``sourceModel`` and
    ``wantAdult`` bound declaratively.
    """

    _IS_ADULT_ROLE = LibraryModel.IsAdultRole

    wantAdultChanged = Signal()

    def __init__(self, want_adult: bool = False, parent=None) -> None:
        super().__init__(parent)
        self._want_adult = bool(want_adult)

    def _get_want_adult(self) -> bool:
        return self._want_adult

    def _set_want_adult(self, value: bool) -> None:
        value = bool(value)
        if value != self._want_adult:
            self._want_adult = value
            self.invalidateFilter()
            self.wantAdultChanged.emit()

    # Which half to keep. Changing it re-runs the filter so QML bindings react.
    wantAdult = Property(bool, _get_want_adult, _set_want_adult, notify=wantAdultChanged)

    def filterAcceptsRow(self, source_row: int, source_parent: QModelIndex) -> bool:
        model = self.sourceModel()
        if model is None:
            return False
        index = model.index(source_row, 0, source_parent)
        return bool(model.data(index, self._IS_ADULT_ROLE)) == self._want_adult

    @property
    def count(self) -> int:
        return self.rowCount()
